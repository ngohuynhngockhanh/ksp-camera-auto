/* ksp-camera-auto — help view: search + chatbox over the generated help
 * bundle (web/static/help/help-index.json, built by `make docs`). Vanilla JS,
 * self-contained: app.js only routes #help to this view. All article HTML is
 * pre-rendered and escaped by tools/docgen, so innerHTML here is safe. */

(function () {
  'use strict';

  let db = null;        // bundle: {sections, articles}
  let corpus = null;    // per-article folded search fields
  let loadFailed = false;

  /* ---------- Vietnamese normalization ----------
   * NFD strips combining accents; đ/Đ do NOT decompose and need an explicit
   * replace. "Mật Khẩu" and "mat khau" fold to the same string. */
  function fold(s) {
    return String(s || '')
      .toLowerCase()
      .normalize('NFD')
      .replace(/[̀-ͯ]/g, '')
      .replace(/đ/g, 'd');
  }
  // Filler words in "tôi muốn đổi..." style queries — pure noise for ranking.
  const STOP = new Set(['toi', 'minh', 'ban', 'muon', 'can', 'lam', 'sao', 'the',
    'nao', 'cach', 'cho', 'va', 'la', 'gi', 'o', 'bi', 'khong', 'duoc', 'giup',
    'hay', 'xin', 'mot', 'cai', 'con', 'thi', 'ma', 'nhu', 'them']);

  function tokens(s) {
    return fold(s).split(/[^a-z0-9+.]+/).filter(t => t.length >= 2 && !STOP.has(t));
  }
  function words(s) {
    return fold(s).split(/[^a-z0-9+.]+/).filter(Boolean);
  }

  /* ---------- data loading ---------- */

  async function loadDb() {
    if (db || loadFailed) return db;
    try {
      const res = await fetch('help/help-index.json');
      if (!res.ok) throw new Error('HTTP ' + res.status);
      db = await res.json();
      corpus = db.articles.map(a => ({
        a,
        titleWords: words(a.title),
        kwPhrases: (a.keywords || []).map(k => words(k).join(' ')).filter(Boolean),
        kwWords: (a.keywords || []).flatMap(words),
        text: fold(a.text),
      }));
    } catch (e) {
      loadFailed = true;
      const box = document.getElementById('help-results');
      if (box) box.innerHTML = '<p class="muted">Không tải được dữ liệu trợ giúp (' +
        escapeHtml(e.message) + '). Hãy build lại với <code>make docs</code>.</p>';
    }
    return db;
  }

  /* ---------- scoring (shared by search + chat) ---------- */

  // Word-level matching: a 2-char token must equal a word ("do" ≠ "doi"),
  // longer tokens may prefix-match ("phan" hits "phan", "wifi" hits "wifi").
  function wordMatch(ws, t) {
    return ws.some(w => w === t || (t.length >= 3 && w.startsWith(t)));
  }

  function scoreArticle(entry, qFold, qTokens, keywordWeight) {
    let score = 0;
    // Strongest signal: a multi-word keyword phrase appearing whole in the
    // query ("đổi độ phân giải" contains keyword "độ phân giải").
    for (const p of entry.kwPhrases) {
      if (p.includes(' ') && qFold.includes(p)) score += 12;
    }
    for (const t of qTokens) {
      if (wordMatch(entry.titleWords, t)) score += 5;
      if (wordMatch(entry.kwWords, t)) score += keywordWeight;
      if (t.length >= 3 && entry.text.includes(t)) score += 1;
    }
    return score;
  }

  function rank(query, keywordWeight, limit) {
    const qTokens = tokens(query);
    if (!qTokens.length) return [];
    const qFold = words(query).join(' ');
    return corpus
      .map(e => ({ e, score: scoreArticle(e, qFold, qTokens, keywordWeight) }))
      .filter(r => r.score > 0)
      .sort((x, y) => y.score - x.score)
      .slice(0, limit);
  }

  /* ---------- search UI ---------- */

  function sectionTitle(id) {
    const s = db.sections.find(s => s.id === id);
    return s ? s.title : id;
  }

  function renderTOC() {
    const box = document.getElementById('help-results');
    let html = '';
    for (const sec of db.sections) {
      const arts = db.articles.filter(a => a.section === sec.id);
      if (!arts.length) continue;
      html += '<div class="help-sec-title">' + escapeHtml(sec.title) + '</div><ul class="help-toc">';
      for (const a of arts) {
        html += '<li><a href="#help/' + escapeHtml(a.id) + '">' + escapeHtml(a.title) + '</a></li>';
      }
      html += '</ul>';
    }
    box.innerHTML = html;
  }

  function highlight(text, qTokens) {
    // Diacritic-insensitive <mark>: fold rune by rune, remembering which
    // original rune produced each folded character, then map matches back.
    const runes = Array.from(text);
    let folded = '';
    const owner = []; // owner[j] = index in runes of folded char j
    runes.forEach((r, i) => {
      const f = fold(r);
      for (let k = 0; k < f.length; k++) { folded += f[k]; owner.push(i); }
    });
    const ranges = [];
    for (const t of qTokens) {
      for (let from = 0; ;) {
        const idx = folded.indexOf(t, from);
        if (idx === -1) break;
        ranges.push([owner[idx], owner[idx + t.length - 1] + 1]);
        from = idx + t.length;
      }
    }
    if (!ranges.length) return escapeHtml(text);
    ranges.sort((a, b) => a[0] - b[0]);
    let out = '', cur = 0;
    for (const [s, e] of ranges) {
      if (s < cur) continue;
      out += escapeHtml(runes.slice(cur, s).join('')) + '<mark>' + escapeHtml(runes.slice(s, e).join('')) + '</mark>';
      cur = e;
    }
    return out + escapeHtml(runes.slice(cur).join(''));
  }

  function renderResults(query) {
    const box = document.getElementById('help-results');
    if (!tokens(query).length) { renderTOC(); return; }
    const hits = rank(query, 3, 10);
    if (!hits.length) {
      box.innerHTML = '<p class="muted">Không tìm thấy. Thử từ khóa khác, ví dụ: mật khẩu, wifi, độ phân giải…</p>';
      return;
    }
    const qTokens = tokens(query);
    box.innerHTML = hits.map(({ e }) =>
      '<a class="help-hit" href="#help/' + escapeHtml(e.a.id) + '">' +
        '<div class="help-hit-title">' + highlight(e.a.title, qTokens) + '</div>' +
        '<div class="help-hit-snippet">' + highlight(e.a.snippet, qTokens) + '</div>' +
      '</a>').join('');
  }

  /* ---------- article view ---------- */

  function renderArticle(id) {
    const pane = document.getElementById('help-article');
    const art = db.articles.find(a => a.id === id);
    if (!art) {
      pane.hidden = false;
      pane.innerHTML = '<p class="muted">Không có bài trợ giúp này.</p>' +
        '<p><a class="btn btn-secondary" href="#help">Quay lại danh sách</a></p>';
      return;
    }
    let html = '<div class="help-art-head">' +
      '<a class="btn btn-secondary btn-sm" href="#help">← Quay lại</a>' +
      '<span class="help-art-sec">' + escapeHtml(sectionTitle(art.section)) + '</span></div>' +
      '<h2 class="help-art-title">' + escapeHtml(art.title) + '</h2>';
    if (art.ui) {
      html += '<p><a class="btn btn-sm" href="' + escapeHtml(art.ui) + '">Mở tính năng</a></p>';
    }
    html += '<div class="help-art-body">' + art.html + '</div>';
    if (art.related && art.related.length) {
      html += '<div class="help-related"><span class="muted">Bài liên quan:</span> ' +
        art.related.map(r => {
          const ra = db.articles.find(a => a.id === r);
          return ra ? '<a href="#help/' + escapeHtml(r) + '">' + escapeHtml(ra.title) + '</a>' : '';
        }).filter(Boolean).join(' · ') + '</div>';
    }
    pane.hidden = false;
    pane.innerHTML = html;
    pane.scrollIntoView({ block: 'start' });
  }

  /* ---------- chatbox ---------- */

  const CHAT_CHIPS = [
    'đổi mật khẩu hàng loạt',
    'quét mạng tìm camera',
    'chỉnh độ phân giải',
    'cấu hình wifi',
    'camera không kết nối được',
  ];

  function chatBubble(cls, html) {
    const log = document.getElementById('chat-log');
    const el = document.createElement('div');
    el.className = 'chat-msg ' + cls;
    el.innerHTML = html;
    log.appendChild(el);
    log.scrollTop = log.scrollHeight;
  }

  function chatAnswer(query) {
    const hits = rank(query, 5, 3);
    if (!hits.length) {
      chatBubble('bot',
        '<p>Mình chưa nhận ra tính năng phù hợp. Bạn thử mô tả khác đi, hoặc xem các mục: ' +
        db.sections.map(s => escapeHtml(s.title)).join(', ') + '.</p>');
      return;
    }
    const cards = hits.map(({ e }) => {
      let card = '<div class="chat-card"><div class="chat-card-title">' + escapeHtml(e.a.title) + '</div>' +
        '<div class="chat-card-snippet">' + escapeHtml(e.a.snippet) + '</div>' +
        '<div class="chat-card-actions">' +
        '<a class="btn btn-secondary btn-sm" href="#help/' + escapeHtml(e.a.id) + '">Xem hướng dẫn</a>';
      if (e.a.ui) card += '<a class="btn btn-sm" href="' + escapeHtml(e.a.ui) + '">Mở tính năng</a>';
      card += '</div></div>';
      return card;
    }).join('');
    chatBubble('bot', '<p>Có thể bạn cần:</p>' + cards);
  }

  async function chatSubmit(text) {
    text = String(text || '').trim();
    if (!text) return;
    if (!await loadDb()) return;
    chatBubble('user', '<p>' + escapeHtml(text) + '</p>');
    chatAnswer(text);
  }

  /* ---------- routing & init ---------- */

  function currentArticleId() {
    const h = location.hash || '';
    return h.startsWith('#help/') ? decodeURIComponent(h.slice(6)) : '';
  }

  async function onRoute() {
    if ((location.hash || '#dashboard').slice(1).split('/')[0] !== 'help') return;
    if (!await loadDb()) return;
    const id = currentArticleId();
    const pane = document.getElementById('help-article');
    if (id) {
      renderArticle(id);
    } else {
      pane.hidden = true;
      renderResults(document.getElementById('help-search').value);
    }
  }

  function init() {
    const search = document.getElementById('help-search');
    let timer = null;
    search.addEventListener('input', () => {
      clearTimeout(timer);
      timer = setTimeout(async () => {
        if (!await loadDb()) return;
        if (currentArticleId()) location.hash = '#help';
        renderResults(search.value);
      }, 120);
    });

    const chips = document.getElementById('chat-chips');
    chips.innerHTML = CHAT_CHIPS.map(c =>
      '<button type="button" class="chip chip-btn" data-chat-chip="' + escapeHtml(c) + '">' + escapeHtml(c) + '</button>').join('');
    chips.addEventListener('click', (ev) => {
      const btn = ev.target.closest('[data-chat-chip]');
      if (btn) chatSubmit(btn.dataset.chatChip);
    });

    document.getElementById('chat-form').addEventListener('submit', (ev) => {
      ev.preventDefault();
      const input = document.getElementById('chat-input');
      chatSubmit(input.value);
      input.value = '';
    });

    window.addEventListener('hashchange', onRoute);
    onRoute();
  }

  init();
})();
