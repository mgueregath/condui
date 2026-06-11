package weblog

import (
	"fmt"
	neturl "net/url"
)

// RenderLogViewerHTML renders the live Docker log viewer page.
func RenderLogViewerHTML(name, session, container string) string {
	streamURL := fmt.Sprintf("/stream?session=%s&container=%s",
		neturl.QueryEscape(session), neturl.QueryEscape(container))

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<title>Logs — %s</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
html,body{height:100%%;background:#08060f;color:#c9d1de;font-family:"JetBrains Mono","Cascadia Code","Fira Code",monospace;font-size:12px;line-height:1.6}
#header{display:flex;align-items:center;justify-content:space-between;padding:10px 16px;background:#0d0b18;border-bottom:1px solid #1e1b2e;position:sticky;top:0;z-index:10;gap:12px}
#title{font-weight:600;font-size:13px;color:#e2e8f0}
#badge{font-size:10px;font-weight:700;letter-spacing:.06em;animation:pulse 1.5s ease-in-out infinite}
.live{color:#4ade80}.stopped{color:#f87171;animation:none!important}
@keyframes pulse{0%%,100%%{opacity:1}50%%{opacity:.3}}
#count{font-size:10px;color:#4b5563}
#controls{display:flex;gap:6px}
button{padding:3px 10px;border-radius:4px;border:1px solid #1e1b2e;background:transparent;color:#94a3b8;font-size:11px;font-family:inherit;cursor:pointer}
button:hover{background:#1e1b2e;color:#e2e8f0}
#follow-btn{border-color:var(--accent,#6366f1);color:#818cf8;display:none}
#logs{padding:10px 16px;min-height:calc(100%% - 45px)}
.line{white-space:pre-wrap;word-break:break-all}
::-webkit-scrollbar{width:6px}::-webkit-scrollbar-track{background:#0d0b18}::-webkit-scrollbar-thumb{background:#1e1b2e;border-radius:3px}
</style>
</head>
<body>
<div id="header">
  <div style="display:flex;align-items:center;gap:10px;min-width:0">
    <span id="title">%s</span>
    <span id="badge" class="live">● LIVE</span>
    <span id="count">0 líneas</span>
  </div>
  <div id="controls">
    <button onclick="document.getElementById('logs').innerHTML='';lineCount=0;updateCount()">Limpiar</button>
    <button id="follow-btn" onclick="enableFollow()">↓ Seguir</button>
  </div>
</div>
<div id="logs"></div>
<script>
var following=true,lineCount=0;
function updateCount(){document.getElementById('count').textContent=lineCount+' líneas'}
function enableFollow(){following=true;document.getElementById('follow-btn').style.display='none';window.scrollTo(0,document.body.scrollHeight)}
window.addEventListener('scroll',function(){
  var atBottom=window.innerHeight+window.scrollY>=document.body.offsetHeight-60;
  following=atBottom;
  document.getElementById('follow-btn').style.display=atBottom?'none':'block';
});
var es=new EventSource('%s');
es.onmessage=function(e){
  if(e.data==='__END__'){
    es.close();
    document.getElementById('badge').textContent='■ STOPPED';
    document.getElementById('badge').className='stopped';
    return;
  }
  var d=document.getElementById('logs');
  var line=document.createElement('div');
  line.className='line';
  line.textContent=e.data;
  d.appendChild(line);
  lineCount++;
  updateCount();
  if(following)window.scrollTo(0,document.body.scrollHeight);
};
es.onerror=function(){
  document.getElementById('badge').textContent='■ STOPPED';
  document.getElementById('badge').className='stopped';
};
</script>
</body>
</html>`, name, name, streamURL)
}
