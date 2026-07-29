(function(){
  const current=document.body&&document.body.dataset.current;
  document.querySelectorAll('[data-nav]').forEach(function(a){
    if(a.dataset.nav===current){a.setAttribute('aria-current','page');a.classList.add('active');}
  });
  document.querySelectorAll('.diagram img').forEach(function(img){
    if(img.parentElement.tagName.toLowerCase()==='figure') return;
    const figure=document.createElement('figure');
    const caption=document.createElement('figcaption');
    caption.textContent=img.alt||'架构示意图';
    figure.className='diagram-figure';
    img.parentNode.insertBefore(figure,img);
    figure.appendChild(img);
    figure.appendChild(caption);
  });
})();
