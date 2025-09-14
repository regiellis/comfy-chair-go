(function(){
  const NS = '{{NodeNameLower}}';
  if (window.__{{NodeNameLower}}_panel) return;
  function el(tag, attrs={}, children=[]) { const e=document.createElement(tag); Object.entries(attrs).forEach(([k,v])=>e.setAttribute(k,v)); children.forEach(c=>e.appendChild(typeof c==='string'?document.createTextNode(c):c)); return e; }
  const panel = el('div'); window.__{{NodeNameLower}}_panel = panel; panel.style.cssText='position:fixed;top:60px;right:20px;z-index:9999;background:#111;color:#eee;font:12px sans-serif;padding:10px;border:1px solid #444;border-radius:6px;min-width:240px;max-width:320px;';
  panel.innerHTML='<div style="font-weight:bold;margin-bottom:6px;cursor:move;">{{NodeName}} Panel</div>' +
    '<div><input id="sp_topic" placeholder="topic" style="width:100%;margin-bottom:4px;"/><button id="sp_go" style="width:100%;">Generate</button></div>' +
    '<pre id="sp_out" style="margin-top:6px;max-height:140px;overflow:auto;background:#000;padding:6px;"></pre>' +
    '<div style="margin-top:8px;border-top:1px solid #333;padding-top:6px;">Batch (comma sep topics)<br/><textarea id="sp_batch" style="width:100%;height:60px;"></textarea><button id="sp_batch_btn" style="width:100%;margin-top:4px;">Start Batch</button><div id="sp_batch_status" style="margin-top:4px;"></div></div>';
  document.body.appendChild(panel);
  (function(){let dx,dy,down=false; const head=panel.firstChild; head.addEventListener('mousedown',e=>{down=true;dx=e.clientX-panel.offsetLeft;dy=e.clientY-panel.offsetTop;}); window.addEventListener('mousemove',e=>{if(!down) return; panel.style.left=(e.clientX-dx)+'px'; panel.style.top=(e.clientY-dy)+'px';}); window.addEventListener('mouseup',()=>down=false);})();
  async function j(r){let js={};try{js=await r.json()}catch{} if(!r.ok||js.success===false) throw new Error(js.error||r.status); return js; }
  async function post(p,b){ return j(await fetch('/'+NS+p,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)})); }
  async function get(p,q={}){ const u=new URL('/'+NS+p,location.origin); Object.entries(q).forEach(([k,v])=>u.searchParams.set(k,v)); return j(await fetch(u)); }
  panel.querySelector('#sp_go').onclick=async()=>{ const out=panel.querySelector('#sp_out'); out.textContent='...'; try{ const r=await post('/generate',{topic:panel.querySelector('#sp_topic').value}); out.textContent=r.result; }catch(e){ out.textContent='ERR '+e.message; } };
  panel.querySelector('#sp_batch_btn').onclick=async()=>{ const raw=panel.querySelector('#sp_batch').value; const topics=raw.split(',').map(s=>s.trim()).filter(Boolean); const {job_id}=await post('/batch_start',{topics}); const stat=panel.querySelector('#sp_batch_status'); stat.textContent='Job '+job_id+' started'; const iv=setInterval(async()=>{ try{ const s=await get('/batch_status',{job_id}); stat.textContent='Status: '+s.data.status+' '+s.data.progress+'%'; if(s.data.status!=='running'){ clearInterval(iv); const r=await get('/batch_results',{job_id}); panel.querySelector('#sp_out').textContent=r.results.join('\n'); } }catch(e){ stat.textContent='ERR '+e.message; clearInterval(iv);} },1200); };
})();
