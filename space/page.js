// humanizer Space: language switch, theme toggle, sample drafts, word counter, copy buttons.
// Runs from <head>; everything is delegated from document, so Gradio re-renders don't matter.
(function () {
  var MAX_WORDS = 700, MAX_CJK = 1200;
  var root = document.documentElement;
  var KEY = 'hz-space-lang';

  function store(k, v) { try { if (v === undefined) return localStorage.getItem(k); localStorage.setItem(k, v); } catch (e) { return null; } }

  var lang = null;
  try { var q = new URLSearchParams(location.search).get('lang'); if (q === 'zh' || q === 'en') lang = q; } catch (e) {}
  if (!lang) lang = store(KEY);
  if (lang !== 'zh' && lang !== 'en') lang = 'en';   // English unless ?lang=zh or the visitor picked 中文 before
  root.dataset.lang = lang;
  root.lang = lang === 'zh' ? 'zh-CN' : 'en';

  var PH = { en: 'Paste the AI-written draft here…', zh: '把 AI 写的草稿贴在这里……' };

  function $(s, el) { return (el || document).querySelector(s); }
  function draftArea() { return $('#draft textarea'); }

  // ── word counter (same units as hz: English words + Chinese characters) ──
  var CJK = /[぀-ヿ㐀-䶿一-鿿豈-﫿가-힯]/g;
  function count(s) {
    var cjk = (s.match(CJK) || []).length;
    var latin = s.replace(CJK, ' ').split(/\s+/).filter(function (t) { return /[\p{L}\p{N}]/u.test(t); }).length;
    return { cjk: cjk, latin: latin };
  }
  function updateCount() {
    var ta = draftArea(), el = $('#draft-count');
    if (!ta || !el) return;
    var c = count(ta.value || ''), zh = c.cjk > c.latin;
    var n = c.cjk + c.latin, size = c.latin + c.cjk * MAX_WORDS / MAX_CJK;
    var unitEn = zh ? 'chars' : 'words', unitZh = '字';
    if (!n) zh = root.dataset.lang === 'zh';
    var lim = zh ? MAX_CJK.toLocaleString('en-US') : String(MAX_WORDS);
    el.innerHTML = '<b>' + n.toLocaleString('en-US') + '</b> <span class="l-en">' + unitEn + '</span><span class="l-zh">' + unitZh +
      '</span> <span class="dim">/ ' + lim + '</span>';
    el.classList.toggle('over', size > MAX_WORDS);
    ta.classList.toggle('cjk', zh);
  }

  function applyLang() {
    var ta = draftArea();
    if (ta) ta.placeholder = PH[root.dataset.lang] || PH.en;
  }
  function setLang(l) {
    root.dataset.lang = l;
    root.lang = l === 'zh' ? 'zh-CN' : 'en';
    store(KEY, l);
    applyLang();
  }

  function setDraft(text) {
    var ta = draftArea();
    if (!ta) return;
    var setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set;
    setter.call(ta, text);
    ta.dispatchEvent(new Event('input', { bubbles: true }));
    updateCount();
  }

  function copyText(text, btn) {
    function done() { if (!btn) return; btn.classList.add('copied'); setTimeout(function () { btn.classList.remove('copied'); }, 1600); }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, function () { fallback(text); done(); });
    } else { fallback(text); done(); }
  }
  function fallback(text) {
    var t = document.createElement('textarea');
    t.value = text; t.style.position = 'fixed'; t.style.opacity = '0';
    document.body.appendChild(t); t.select();
    try { document.execCommand('copy'); } catch (e) {}
    t.remove();
  }

  document.addEventListener('click', function (e) {
    var a = e.target.closest('a[href^="#"]');
    if (a && a.getAttribute('href').length > 1) {
      // On huggingface.co the Space sits in an auto-height iframe, so a plain #hash jump scrolls nothing.
      // scrollIntoView also scrolls the parent page.
      var dest = document.getElementById(a.getAttribute('href').slice(1));
      if (dest) { e.preventDefault(); dest.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
      return;
    }
    var t = e.target.closest('[data-setlang],[data-act],[data-sample]');
    if (!t) return;
    if (t.dataset.setlang) { setLang(t.dataset.setlang); return; }
    if (t.dataset.sample) {
      var s = (window.HZ_SAMPLES || {})[t.dataset.sample];
      if (s) {
        setDraft(s);
        if (t.closest('#hz-body')) { var ed = $('#editor'); if (ed) ed.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
      }
      return;
    }
    switch (t.dataset.act) {
      case 'theme': document.body.classList.toggle('dark'); break;
      case 'clear': setDraft(''); var ta = draftArea(); if (ta) ta.focus(); break;
      case 'diff': root.dataset.diff = root.dataset.diff === 'off' ? 'on' : 'off'; break;
      case 'copy-out': {
        var pre = t.closest('.out-wrap') && t.closest('.out-wrap').querySelector('pre.plain');
        if (pre) copyText(pre.textContent, t);
        break;
      }
      case 'copy-code': {
        var code = t.closest('.code') && t.closest('.code').querySelector('code');
        if (code) copyText(code.textContent, t);
        break;
      }
    }
  });

  document.addEventListener('input', function (e) {
    if (e.target && e.target.closest && e.target.closest('#draft')) updateCount();
  });

  document.addEventListener('keydown', function (e) {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && e.target.closest && e.target.closest('#draft')) {
      e.preventDefault();
      var go = $('#go');
      if (go) go.click();
    }
  });

  // On a direct *.hf.space visit Hugging Face pins its own badge (#huggingface-space-header) over the
  // top-right corner; move the bar below it so the language switch stays reachable.
  [500, 1500, 3000, 6000].forEach(function (ms) {
    setTimeout(function () {
      if (document.getElementById('huggingface-space-header')) root.classList.add('hz-hfbadge');
    }, ms);
  });

  // Gradio renders after this script runs: wait for the textarea once, then set placeholder and counter.
  var tries = 0;
  var iv = setInterval(function () {
    tries += 1;
    if (draftArea() && document.getElementById('ex-4')) {
      applyLang(); updateCount();
      if (root.dataset.lang === 'zh') document.getElementById('ex-4').checked = true;  // open on a Chinese example
      var k = $('.go-kbd');
      if (k && !/Mac|iPhone|iPad/.test(navigator.platform || '')) k.textContent = 'Ctrl ↵';
      clearInterval(iv);
    } else if (tries > 300) clearInterval(iv);
  }, 100);
})();
