// 在样式生效前定好主题和语言,避免闪一下。(CSP 不允许内联脚本,所以单独一个文件)
(function () {
  var d = document.documentElement, q = new URLSearchParams(location.search), s = {};
  try { s.theme = localStorage.getItem('hz.theme'); s.lang = localStorage.getItem('hz.lang'); } catch (e) {}
  var theme = q.get('theme') || s.theme;
  if (theme === 'light' || theme === 'dark') d.setAttribute('data-theme', theme);
  var lang = q.get('lang') || s.lang || ((navigator.language || '').toLowerCase().indexOf('zh') === 0 ? 'zh' : 'en');
  d.lang = lang === 'zh' ? 'zh-CN' : 'en';
  d.setAttribute('data-lang', lang === 'zh' ? 'zh' : 'en');
  if (/Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)) d.classList.add('mac');
})();
