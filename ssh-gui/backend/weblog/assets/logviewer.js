let following = true;
let lineCount = 0;

function updateCount() {
    document.getElementById('count').textContent =
        lineCount + ' líneas';
}

function clearLogs() {
    document.getElementById('logs').innerHTML = '';
    lineCount = 0;
    updateCount();
}

function enableFollow() {
    following = true;

    document.getElementById('follow-btn').style.display =
        'none';

    window.scrollTo(
        0,
        document.body.scrollHeight
    );
}

window.addEventListener('scroll', function () {

    const atBottom =
        window.innerHeight +
        window.scrollY >=
        document.body.offsetHeight - 60;

    following = atBottom;

    document.getElementById('follow-btn').style.display =
        atBottom ? 'none' : 'block';
});

function escapeHtml(str) {

    return str
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#039;');
}

function extractTimestamp(text) {

    const patterns = [

        // 2026-06-15T13:45:21Z
        /^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?/,
        // 13:45:21
        /^\d{2}:\d{2}:\d{2}(?:\.\d+)?/,
        // [2026-06-15T13:45:21Z]
        /^\[[0-9:\-T.Z+]+\]/,
        // 2026-06-15 13:45:21
        /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?/,
        // 2026.06.15 13:45:21
        /^\d{4}\.\d{2}\.\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?/,
        // 2026-06-15
        /^\d{4}-\d{2}-\d{2}/,
        // 2026/06/13 12:05PM
        /^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}(?:AM|PM)?/,
        // [2026-05-07 09:16:49.448 +0000]
        /\[[0-9\-]+ [0-9:.]+ [\+0-9]+\]/,
    ];

    for (const pattern of patterns) {

        const match = text.match(pattern);

        if (match) {

            return {
                timestamp: match[0],
                remainder: text.substring(match[0].length).trim(),
            };
        }
    }

    return {
        timestamp: null,
        remainder: text,
    };
}

function detectLevel(text) {

  const match = text.match(
    /\b(FATAL|ERROR|ERR|WARNING|WARN|WRN|DETAIL|INFO|INF|LOG|DEBUG|TRACE)\b/i
  );

  if (!match)
    return [null, null, null];

  const level = match[1].toUpperCase();

  switch (level) {

    case 'FATAL':
      return ['FATL', 'level-error', level];

    case 'ERROR':
    case 'ERR':
      return ['ERRO', 'level-error', level];

    case 'WARNING':
    case 'WARN':
    case 'WRN':
    case 'DETAIL':
      return ['WARN', 'level-warning', level];

    case 'INFO':
    case 'INF':
    case 'LOG':
      return ['INFO', 'level-info', level];

    case 'DEBUG':
      return ['DEBG', 'level-debug', level];

    case 'TRACE':
      return ['TRCE', 'level-trace', level];
  }

  return [null, null, null];
}

function renderLogLine(raw) {

    const ts = extractTimestamp(raw);

    const timestamp = ts.timestamp;
    const text = ts.remainder;

/*     const [level, levelClass] =
        detectLevel(text); */
    const [displayLevel, levelClass, level] =
      detectLevel(text);

    let html = '';

    if (timestamp) {

        html +=
            '<span class="timestamp">' +
            escapeHtml(timestamp) +
            '</span> ';
    }

    if (level) {

        /* html +=
            '<span class="' +
            levelClass +
            '">[' +
            level +
            ']</span> '; */
        html +=
            '<span class="' +
            levelClass +
            '">[' +
            displayLevel +
            ']</span> ';
    }

    html +=
        '<span class="message">' +
        escapeHtml(text) +
        '</span>';

    return html;
}

const es = new EventSource(
    window.LOG_STREAM_URL
);

es.onmessage = function (e) {

    if (e.data === '__END__') {

        es.close();

        document.getElementById('badge').textContent =
            '■ STOPPED';

        document.getElementById('badge').className =
            'stopped';

        return;
    }

    const logs =
        document.getElementById('logs');

    const line =
        document.createElement('div');

    line.className = 'line';

    line.innerHTML =
        renderLogLine(e.data);

    logs.appendChild(line);

    lineCount++;

    updateCount();

    if (following) {

        window.scrollTo(
            0,
            document.body.scrollHeight
        );
    }
};

es.onerror = function () {

    document.getElementById('badge').textContent =
        '■ STOPPED';

    document.getElementById('badge').className =
        'stopped';
};