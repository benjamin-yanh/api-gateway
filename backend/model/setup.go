package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrSetupAlreadyCompleted = errors.New("system setup is already completed")

type Setup struct {
	ID            uint   `json:"id" gorm:"primaryKey"`
	Version       string `json:"version" gorm:"type:varchar(50);not null"`
	InitializedAt int64  `json:"initialized_at" gorm:"type:bigint;not null"`
}

// CompleteInitialSetup claims the singleton setup row before creating the root
// user and options. The primary-key insert is the cross-database serialization
// point: only one concurrent transaction can claim ID 1.
func CompleteInitialSetup(rootUser *User, setup Setup, options map[string]string) error {
	setup.ID = 1
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&setup)
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected != 1 {
			return ErrSetupAlreadyCompleted
		}
		var rootCount int64
		if err := tx.Model(&User{}).Where("role = ?", common.RoleRootUser).Count(&rootCount).Error; err != nil {
			return err
		}
		if rootCount == 0 && rootUser != nil {
			if err := tx.Create(rootUser).Error; err != nil {
				return err
			}
		}
		for key, value := range options {
			option := Option{Key: key}
			if err := tx.FirstOrCreate(&option, Option{Key: key}).Error; err != nil {
				return err
			}
			option.Value = value
			if err := tx.Save(&option).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for key, value := range options {
		if err := updateOptionMap(key, value); err != nil {
			return err
		}
	}
	return nil
}

func GetSetup() *Setup {
	var setup Setup
	err := DB.First(&setup).Error
	if err != nil {
		return nil
	}
	return &setup
}
