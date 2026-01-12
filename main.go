package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"DigitalMarket/history"
	"DigitalMarket/realtime"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	priceMgr := realtime.NewManager()

	// ====== API：实时价（WS）======
	r.GET("/price", func(c *gin.Context) {
		symbol := c.DefaultQuery("symbol", "BTCUSDT")
		price := priceMgr.GetPrice(symbol)
		if price == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "price not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"symbol": symbol, "price": price})
	})

	// ====== API：历史K线 JSON ======
	r.GET("/klines", func(c *gin.Context) {
		symbol := c.DefaultQuery("symbol", "BTCUSDT")
		interval := c.DefaultQuery("interval", "1h")

		end := time.Now().UTC()
		start := end.Add(-30 * 24 * time.Hour)

		if s := c.Query("start"); s != "" {
			if t, ok := history.ParseTimeFlexible(s); ok {
				start = t.UTC()
			}
		}
		if e := c.Query("end"); e != "" {
			if t, ok := history.ParseTimeFlexible(e); ok {
				end = t.UTC()
			}
		}

		klines, err := history.FetchKlines(symbol, interval, start, end)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"symbol":   symbol,
			"interval": interval,
			"count":    len(klines),
			"data":     klines,
		})
	})

	// ====== 可视化页面 ======
	r.GET("/klines/view", func(c *gin.Context) {
		symbol := c.DefaultQuery("symbol", "BTCUSDT")
		interval := c.DefaultQuery("interval", "1h")

		// 默认：最近 7 天（更像交易软件）
		end := time.Now().UTC()
		start := end.Add(-7 * 24 * time.Hour)

		if s := c.Query("start"); s != "" {
			if t, ok := history.ParseTimeFlexible(s); ok {
				start = t.UTC()
			}
		}
		if e := c.Query("end"); e != "" {
			if t, ok := history.ParseTimeFlexible(e); ok {
				end = t.UTC()
			}
		}

		klines, err := history.FetchKlines(symbol, interval, start, end)
		if err != nil {
			c.String(500, "error: %v", err)
			return
		}

		dataJSON, _ := json.Marshal(klines)

		tpl := template.Must(template.New("kview").Parse(pageHTML))
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tpl.Execute(c.Writer, gin.H{
			"Symbol":   symbol,
			"Interval": interval,
			"Start":    start.Format("2006-01-02"),
			"End":      end.Format("2006-01-02"),
			"Klines":   template.JS(dataJSON),
		})
	})

	_ = r.Run(":8080")
}

const pageHTML = `
<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width,initial-scale=1"/>
  <title>Klines Viewer</title>
  <script src="https://cdn.plot.ly/plotly-2.27.0.min.js"></script>
  <style>
    :root { --bd:#e5e7eb; --fg:#111827; --muted:#6b7280; --bg:#ffffff; }
    body { font-family: system-ui,-apple-system,Segoe UI,Roboto,Arial; margin: 16px; color:var(--fg); background:var(--bg); }
    .row { display:flex; flex-wrap:wrap; gap:10px; align-items:end; }
    .card { border:1px solid var(--bd); border-radius:12px; padding:12px; }
    label { font-size:12px; color:var(--muted); display:block; margin-bottom:4px; }
    input, select, button { padding:8px 10px; border:1px solid var(--bd); border-radius:10px; font-size:14px; }
    button { cursor:pointer; background:#111827; color:white; border:none; }
    button.secondary { background:white; color:#111827; border:1px solid var(--bd); }
    .pillbar button { border-radius:999px; padding:7px 10px; }
    .meta { color:var(--muted); font-size:12px; margin-top:8px; }
    #chart { width: 100%; height: 78vh; }
    .split { display:grid; grid-template-columns: 1fr; gap:12px; }
    @media(min-width: 1024px){
      .split { grid-template-columns: 1.2fr 1fr; }
    }
    .checks { display:flex; gap:12px; flex-wrap:wrap; }
    .checks label { display:flex; align-items:center; gap:8px; margin:0; color:var(--fg); font-size:13px; }
    .checks input { width:auto; }
    .small { width:90px; }
  </style>
</head>
<body>
  <h2 style="margin:0 0 10px 0;">Binance Klines 可视化</h2>

  <div class="split">
    <div class="card">
      <div class="row">
        <div>
          <label>Symbol</label>
          <input id="symbol" value="{{.Symbol}}" placeholder="BTCUSDT" />
        </div>
        <div>
          <label>Interval</label>
          <select id="interval">
            <option>1m</option><option>3m</option><option>5m</option><option>15m</option><option>30m</option>
            <option>1h</option><option>2h</option><option>4h</option><option>6h</option><option>12h</option>
            <option>1d</option><option>1w</option>
          </select>
        </div>
        <div>
          <label>Start</label>
          <input id="start" type="date" value="{{.Start}}"/>
        </div>
        <div>
          <label>End</label>
          <input id="end" type="date" value="{{.End}}"/>
        </div>
        <div class="pillbar">
          <label>快捷范围</label>
          <div class="row" style="gap:8px;">
            <button class="secondary" onclick="setRangeDays(1)">1D</button>
            <button class="secondary" onclick="setRangeDays(7)">7D</button>
            <button class="secondary" onclick="setRangeDays(30)">30D</button>
            <button class="secondary" onclick="setRangeDays(90)">90D</button>
          </div>
        </div>
        <div>
          <button onclick="applyAndReload()">加载/刷新</button>
        </div>
        <div>
          <button class="secondary" onclick="toggleAutoRefresh()" id="autoBtn">自动刷新：关</button>
        </div>
      </div>

      <div class="meta">
        当前：<span id="priceNow">-</span>
        <span id="status" style="margin-left:10px;"></span>
      </div>
    </div>

    <div class="card">
      <div style="font-weight:600; margin-bottom:8px;">指标</div>

      <div class="checks">
        <label><input type="checkbox" id="maOn" checked/> MA</label>
        <label><input type="checkbox" id="emaOn"/> EMA</label>
        <label><input type="checkbox" id="rsiOn"/> RSI</label>
        <label><input type="checkbox" id="macdOn"/> MACD</label>
      </div>

      <div class="row" style="margin-top:10px;">
        <div>
          <label>MA 周期</label>
          <input id="maLen" class="small" type="number" value="20" min="2" max="300"/>
        </div>
        <div>
          <label>EMA 周期</label>
          <input id="emaLen" class="small" type="number" value="21" min="2" max="300"/>
        </div>
        <div>
          <label>RSI 周期</label>
          <input id="rsiLen" class="small" type="number" value="14" min="2" max="200"/>
        </div>
        <div>
          <label>MACD Fast</label>
          <input id="macdFast" class="small" type="number" value="12" min="2" max="200"/>
        </div>
        <div>
          <label>MACD Slow</label>
          <input id="macdSlow" class="small" type="number" value="26" min="2" max="300"/>
        </div>
        <div>
          <label>MACD Signal</label>
          <input id="macdSignal" class="small" type="number" value="9" min="2" max="200"/>
        </div>
        <div>
          <button class="secondary" onclick="renderAll()">应用指标</button>
        </div>
      </div>

      <div class="meta">
        说明：MA/EMA 画在价格图上；RSI、MACD 显示在下方子图。
      </div>
    </div>
  </div>

  <div class="card" style="margin-top:12px;">
    <div id="chart"></div>
  </div>

<script>
  // 初始数据（服务端已拉好）
  let klines = {{.Klines}};
  let autoTimer = null;

  // 初始化 interval select
  (function init(){
    document.getElementById('interval').value = "{{.Interval}}";
    renderAll();
    refreshPriceOnce();
  })();

  function setRangeDays(days){
    const end = new Date();
    const start = new Date(end.getTime() - days*24*3600*1000);
    document.getElementById('end').value = fmtDate(end);
    document.getElementById('start').value = fmtDate(start);
  }

  function fmtDate(d){
    const yyyy = d.getFullYear();
    const mm = String(d.getMonth()+1).padStart(2,'0');
    const dd = String(d.getDate()).padStart(2,'0');
    return yyyy+"-"+mm+"-"+dd;
  }

  function applyAndReload(){
    const symbol = document.getElementById('symbol').value.trim() || "BTCUSDT";
    const interval = document.getElementById('interval').value;
    const start = document.getElementById('start').value;
    const end = document.getElementById('end').value;

    const url = new URL(window.location.href);
    url.searchParams.set("symbol", symbol);
    url.searchParams.set("interval", interval);
    if(start) url.searchParams.set("start", start);
    if(end) url.searchParams.set("end", end);
    window.location.href = url.toString();
  }

  async function refreshPriceOnce(){
    const symbol = document.getElementById('symbol').value.trim() || "BTCUSDT";
    try{
      const res = await fetch("/price?symbol="+encodeURIComponent(symbol), {cache:"no-store"});
      if(!res.ok) throw new Error("price http "+res.status);
      const j = await res.json();
      document.getElementById('priceNow').textContent = j.price ? (j.price + " ("+j.symbol+")") : "-";
    }catch(e){
      document.getElementById('priceNow').textContent = "-";
    }
  }

  function toggleAutoRefresh(){
    const btn = document.getElementById("autoBtn");
    if(autoTimer){
      clearInterval(autoTimer);
      autoTimer = null;
      btn.textContent = "自动刷新：关";
      return;
    }
    btn.textContent = "自动刷新：开";
    autoTimer = setInterval(()=>{
      refreshPriceOnce();
      // 仅刷新价格；K线频繁刷新意义不大且更耗 API
    }, 3000);
  }

  function toNum(x){ const n = parseFloat(x); return Number.isFinite(n) ? n : null; }
  function toISO(ms){ return new Date(ms).toISOString(); }

  function sma(arr, len){
    const out = new Array(arr.length).fill(null);
    let sum = 0, cnt = 0;
    for(let i=0;i<arr.length;i++){
      const v = arr[i];
      if(v == null){ out[i]=null; continue; }
      sum += v; cnt++;
      if(i >= len){
        const old = arr[i-len];
        if(old != null){ sum -= old; cnt--; }
      }
      if(i >= len-1 && cnt === len) out[i] = sum/len;
    }
    return out;
  }

  function ema(arr, len){
    const out = new Array(arr.length).fill(null);
    const k = 2/(len+1);
    let prev = null;
    for(let i=0;i<arr.length;i++){
      const v = arr[i];
      if(v == null) { out[i]=null; continue; }
      if(prev == null){
        prev = v;
        out[i] = v;
      }else{
        prev = v*k + prev*(1-k);
        out[i] = prev;
      }
    }
    return out;
  }

  function rsi(close, len){
    const out = new Array(close.length).fill(null);
    let gain=0, loss=0;
    for(let i=1;i<close.length;i++){
      const ch = close[i] - close[i-1];
      const g = Math.max(0, ch);
      const l = Math.max(0, -ch);
      if(i <= len){
        gain += g; loss += l;
        if(i === len){
          const rs = (loss === 0) ? 100 : (gain/loss);
          out[i] = 100 - (100/(1+rs));
        }
      }else{
        gain = (gain*(len-1) + g)/len;
        loss = (loss*(len-1) + l)/len;
        const rs = (loss === 0) ? 100 : (gain/loss);
        out[i] = 100 - (100/(1+rs));
      }
    }
    return out;
  }

  function macd(close, fast, slow, signal){
    const fastE = ema(close, fast);
    const slowE = ema(close, slow);
    const line = close.map((_,i)=>{
      if(fastE[i]==null || slowE[i]==null) return null;
      return fastE[i]-slowE[i];
    });
    const sig = ema(line.map(v=>v==null?0:v), signal).map((v,i)=> line[i]==null ? null : v);
    const hist = line.map((v,i)=>{
      if(v==null || sig[i]==null) return null;
      return v - sig[i];
    });
    return {line, sig, hist};
  }

  function renderAll(){
    if(!klines || klines.length===0){
      Plotly.purge('chart');
      document.getElementById("status").textContent = "无数据";
      return;
    }
    document.getElementById("status").textContent = "数据量: "+klines.length;

    const x = klines.map(k => toISO(k.openTime));
    const open = klines.map(k => toNum(k.open));
    const high = klines.map(k => toNum(k.high));
    const low  = klines.map(k => toNum(k.low));
    const close= klines.map(k => toNum(k.close));
    const vol  = klines.map(k => toNum(k.volume));

    const traces = [];

    // 主图：蜡烛
    traces.push({
      type:'candlestick',
      x, open, high, low, close,
      name:'OHLC',
      xaxis:'x',
      yaxis:'y'
    });

    // 指标：MA/EMA（叠加主图）
    const maOn = document.getElementById('maOn').checked;
    const emaOn = document.getElementById('emaOn').checked;
    const maLen = parseInt(document.getElementById('maLen').value || "20", 10);
    const emaLen = parseInt(document.getElementById('emaLen').value || "21", 10);

    if(maOn){
      const ma = sma(close, maLen);
      traces.push({ type:'scatter', mode:'lines', x, y: ma, name:'MA('+maLen+')', xaxis:'x', yaxis:'y' });
    }
    if(emaOn){
      const e = ema(close, emaLen);
      traces.push({ type:'scatter', mode:'lines', x, y: e, name:'EMA('+emaLen+')', xaxis:'x', yaxis:'y' });
    }

    // 成交量（y2）
    traces.push({
      type:'bar',
      x, y: vol,
      name:'Volume',
      xaxis:'x',
      yaxis:'y2',
      opacity: 0.35
    });

    // RSI（y3）
    const rsiOn = document.getElementById('rsiOn').checked;
    const rsiLen = parseInt(document.getElementById('rsiLen').value || "14", 10);
    if(rsiOn){
      const rv = rsi(close, rsiLen);
      traces.push({ type:'scatter', mode:'lines', x, y: rv, name:'RSI('+rsiLen+')', xaxis:'x', yaxis:'y3' });
    }

    // MACD（y4）
    const macdOn = document.getElementById('macdOn').checked;
    const fast = parseInt(document.getElementById('macdFast').value || "12", 10);
    const slow = parseInt(document.getElementById('macdSlow').value || "26", 10);
    const sigN = parseInt(document.getElementById('macdSignal').value || "9", 10);
    if(macdOn){
      const m = macd(close, fast, slow, sigN);
      traces.push({ type:'scatter', mode:'lines', x, y: m.line, name:'MACD', xaxis:'x', yaxis:'y4' });
      traces.push({ type:'scatter', mode:'lines', x, y: m.sig, name:'Signal', xaxis:'x', yaxis:'y4' });
      traces.push({ type:'bar', x, y: m.hist, name:'Hist', xaxis:'x', yaxis:'y4', opacity:0.35 });
    }

    const layout = {
      margin:{l:60, r:20, t:10, b:40},
      legend:{orientation:'h'},
      xaxis:{rangeslider:{visible:false}},
      // 主图（价格）
      yaxis:{title:'Price', domain:[0.36, 1]},
      // 成交量
      yaxis2:{title:'Volume', domain:[0.24, 0.34]},
      // RSI
      yaxis3:{title:'RSI', domain:[0.12, 0.22], range:[0,100]},
      // MACD
      yaxis4:{title:'MACD', domain:[0.0, 0.10]}
    };

    // 如果 RSI/MACD 没开，把空间让出来（更像专业软件）
    if(!rsiOn && !macdOn){
      layout.yaxis.domain = [0.22, 1];
      layout.yaxis2.domain = [0.0, 0.18];
      delete layout.yaxis3;
      delete layout.yaxis4;
      // 过滤掉 RSI/MACD trace
      const keep = [];
      for(const t of traces){
        if(t.yaxis === 'y3' || t.yaxis === 'y4') continue;
        keep.push(t);
      }
      Plotly.newPlot('chart', keep, layout, {responsive:true});
      return;
    }
    if(rsiOn && !macdOn){
      layout.yaxis.domain = [0.30, 1];
      layout.yaxis2.domain = [0.16, 0.28];
      layout.yaxis3.domain = [0.0, 0.12];
      delete layout.yaxis4;
      const keep = [];
      for(const t of traces){
        if(t.yaxis === 'y4') continue;
        keep.push(t);
      }
      Plotly.newPlot('chart', keep, layout, {responsive:true});
      return;
    }
    if(!rsiOn && macdOn){
      layout.yaxis.domain = [0.30, 1];
      layout.yaxis2.domain = [0.16, 0.28];
      layout.yaxis4.domain = [0.0, 0.12];
      delete layout.yaxis3;
      const keep = [];
      for(const t of traces){
        if(t.yaxis === 'y3') continue;
        keep.push(t);
      }
      Plotly.newPlot('chart', keep, layout, {responsive:true});
      return;
    }

    Plotly.newPlot('chart', traces, layout, {responsive:true});
  }
</script>
</body>
</html>
`
