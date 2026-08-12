package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	_ "github.com/QuantumNous/new-api/setting/performance_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	_ "net/http/pprof"
)

type serverRole string

const (
	serverRoleAll     serverRole = "all"
	serverRoleControl serverRole = "control"
	serverRoleRelay   serverRole = "relay"
)

// DefaultServerRole is overridden with -ldflags for the split backend images.
// The environment variable SERVER_ROLE takes precedence at runtime.
var DefaultServerRole = string(serverRoleAll)

// LockedServerRole is set only for split binaries. When non-empty, runtime
// configuration cannot turn a relay executable into a control-plane server.
var LockedServerRole string

func resolveServerRole() (serverRole, error) {
	configuredRole := serverRole(strings.ToLower(strings.TrimSpace(os.Getenv("SERVER_ROLE"))))
	lockedRole := serverRole(strings.ToLower(strings.TrimSpace(LockedServerRole)))
	if lockedRole != "" {
		if configuredRole != "" && configuredRole != lockedRole {
			return "", fmt.Errorf("SERVER_ROLE %q cannot override locked executable role %q", configuredRole, lockedRole)
		}
		configuredRole = lockedRole
	}

	role := configuredRole
	if role == "" {
		role = serverRole(strings.ToLower(strings.TrimSpace(DefaultServerRole)))
	}
	switch role {
	case serverRoleAll, serverRoleControl, serverRoleRelay:
		return role, nil
	default:
		return "", fmt.Errorf("invalid SERVER_ROLE %q: expected all, control, or relay", role)
	}
}

func (role serverRole) hasControlPlane() bool {
	return role == serverRoleAll || role == serverRoleControl
}

func (role serverRole) hasDataPlane() bool {
	return role == serverRoleAll || role == serverRoleRelay
}

func main() {
	startTime := time.Now()
	role, err := resolveServerRole()
	if err != nil {
		log.Fatal(err)
	}
	if role == serverRoleRelay {
		// Relay nodes never own schema migration or control-plane leader jobs.
		_ = os.Setenv("NODE_TYPE", "slave")
	}
	kitutil.SetLogging(common.SysLog, func(message string) {
		logger.LogError(nil, message)
	})
	kitutil.SetSystemErrorLogging(common.SysError)

	err = InitResources(role)
	if err != nil {
		common.FatalLog("failed to initialize resources: " + err.Error())
		return
	}

	common.SysLog(fmt.Sprintf("New API %s started with server role %s", common.Version, role))
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if common.DebugEnabled {
		common.SysLog("running in debug mode")
	}

	kitutil.Debug.Store(common.DebugEnabled)

	defer func() {
		err := model.CloseDB()
		if err != nil {
			common.FatalLog("failed to close database: " + err.Error())
		}
	}()

	if common.RedisEnabled {
		// for compatibility with old versions
		common.MemoryCacheEnabled = true
	}
	if common.MemoryCacheEnabled {
		common.SysLog("memory cache enabled")
		common.SysLog(fmt.Sprintf("sync frequency: %d seconds", common.SyncFrequency))

		// Add panic recovery and retry for InitChannelCache
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
					// Retry once
					_, _, fixErr := model.FixAbility()
					if fixErr != nil {
						common.FatalLog(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
					}
				}
			}()
			model.InitChannelCache()
		}()

		go model.SyncChannelCache(common.SyncFrequency)
	}

	// Warm pricing after channel cache initialization so Advanced Custom
	// endpoint inference can read cached route settings on first request.
	model.GetPricing()

	// 热更新配置
	go model.SyncOptions(common.SyncFrequency)

	if role.hasControlPlane() {
		// 周期性重载授权策略，保证多节点/多 master 部署下权限变更能传播到每个实例
		go authz.StartPolicySync(common.SyncFrequency)
	}

	if role.hasDataPlane() {
		// 数据看板聚合发生在产生模型用量的数据面。
		go model.UpdateQuotaData()
	}

	if role.hasControlPlane() && os.Getenv("CHANNEL_UPDATE_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_UPDATE_FREQUENCY"))
		if err != nil {
			common.FatalLog("failed to parse CHANNEL_UPDATE_FREQUENCY: " + err.Error())
		}
		go controller.AutomaticallyUpdateChannels(frequency)
	}

	// Codex credential auto-refresh check every 10 minutes, refresh when expires within 1 day
	if role.hasControlPlane() {
		service.StartCodexCredentialAutoRefreshTask()

		// Subscription quota reset task (daily/weekly/monthly/custom)
		service.StartSubscriptionQuotaResetTask()
	}

	// Report this process as a system instance so the System Info page can show
	// all currently alive nodes in multi-instance deployments.
	service.StartSystemInstanceReporter()

	if role.hasControlPlane() {
		// Wire task polling adaptor factory (breaks service -> relay import cycle).
		// Must run before the system task runner starts: the async_task_poll handler
		// calls service.RunTaskPollingOnce, which needs this factory set.
		service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
			a := relay.GetTaskAdaptor(platform)
			if a == nil {
				return nil
			}
			return a
		}

		// Register control-plane scheduled jobs once. Relay replicas never run them.
		controller.RegisterScheduledSystemTasks()
		service.StartSystemTaskRunner()
	}

	if role.hasDataPlane() && os.Getenv("BATCH_UPDATE_ENABLED") == "true" {
		common.BatchUpdateEnabled = true
		common.SysLog("batch update enabled with interval " + strconv.Itoa(common.BatchUpdateInterval) + "s")
		model.InitBatchUpdater()
	}

	if os.Getenv("ENABLE_PPROF") == "true" {
		gopool.Go(func() {
			log.Println(http.ListenAndServe("0.0.0.0:8005", nil))
		})
		go common.Monitor()
		common.SysLog("pprof enabled")
	}

	// Initialize HTTP server
	server := gin.New()
	if err := middleware.ConfigureTrustedProxies(server); err != nil {
		common.FatalLog("failed to configure trusted proxies: " + err.Error())
		return
	}
	server.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		common.SysLog(fmt.Sprintf("panic detected: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Panic detected, error: %v. Please submit a issue here: https://github.com/Calcium-Ion/new-api", err),
				"type":    "new_api_panic",
			},
		})
	}))
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.RequestId())
	server.Use(middleware.Version())
	server.Use(middleware.I18n())
	server.Use(middleware.AccessLog())
	middleware.SetUpLogger(server)
	server.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "role": role})
	})

	switch role {
	case serverRoleControl:
		router.SetControlPlaneRouter(server)
	case serverRoleRelay:
		router.SetDataPlaneRouter(server)
	default:
		router.SetRouter(server, router.WebAssets{
			BuildFS:   buildFS,
			IndexPage: indexPage,
		})
	}
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}
	listenAddress := ":" + port
	if bindAddress := strings.TrimSpace(os.Getenv("BIND_ADDRESS")); bindAddress != "" {
		listenAddress = net.JoinHostPort(bindAddress, port)
	}

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: server,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			common.FatalLog("failed to start HTTP server: " + err.Error())
		}
	}()

	time.Sleep(100 * time.Millisecond)

	common.LogStartupSuccess(startTime, port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	common.SysLog(fmt.Sprintf("received signal: %v, shutting down...", sig))

	// SSE streams may run for minutes; give them time to finish before forced exit
	shutdownTimeout := time.Duration(common.GetEnvOrDefault("SHUTDOWN_TIMEOUT_SECONDS", 120)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		common.SysError(fmt.Sprintf("server forced to shutdown: %v", err))
	}
	// 内存中的看板数据保存入库，避免重启丢失未落库数据 (issue #5679)
	if role.hasDataPlane() && common.DataExportEnabled {
		model.SaveQuotaDataCache()
	}
	common.SysLog("server exited")
}

func InitResources(role serverRole) error {
	// Initialize resources here if needed
	// This is a placeholder function for future resource initialization
	err := godotenv.Load(".env")
	if err != nil {
		if common.DebugEnabled {
			common.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
		}
	}

	// 加载环境变量
	common.InitEnv()

	logger.SetupLogger()

	// Initialize model settings
	ratio_setting.InitRatioSettings()

	service.InitHttpClient()

	service.InitTokenEncoders()

	// Initialize SQL Database
	err = model.InitDB()
	if err != nil {
		common.FatalLog("failed to initialize database: " + err.Error())
		return err
	}
	if role.hasControlPlane() {
		if err = authz.Init(model.DB); err != nil {
			common.FatalLog("failed to initialize authorization: " + err.Error())
			return err
		}
	}

	model.CheckSetup()

	// Initialize options, should after model.InitDB()
	if common.IsMasterNode {
		if err := model.MigrateRetiredFrontendOptions(); err != nil {
			common.SysError("failed to migrate retired frontend options: " + err.Error())
		}
	}
	model.InitOptionMap()

	// 清理旧的磁盘缓存文件
	common.CleanupOldCacheFiles()

	// Initialize SQL Database
	err = model.InitLogDB()
	if err != nil {
		return err
	}

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		return err
	}

	perfmetrics.Init()

	// 启动系统监控
	common.StartSystemMonitor()

	// Initialize i18n
	err = i18n.Init()
	if err != nil {
		common.SysError("failed to initialize i18n: " + err.Error())
		// Don't return error, i18n is not critical
	} else {
		common.SysLog("i18n initialized with languages: " + strings.Join(i18n.SupportedLanguages(), ", "))
	}
	// Register user language loader for lazy loading
	i18n.SetUserLangLoader(model.GetUserLanguage)

	if role.hasControlPlane() {
		// Load custom OAuth providers from database only in the control plane.
		err = oauth.LoadCustomProviders()
		if err != nil {
			common.SysError("failed to load custom OAuth providers: " + err.Error())
			// Don't return error, custom OAuth is not critical
		}

		service.StartAuthArtifactCleanup()
	}

	return nil
}
