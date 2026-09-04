package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	mu        sync.Mutex
	materials = []map[string]string{
		{"id": "1", "name": "Cimento CP-II 50kg", "category": "Construção", "qty": "120"},
		{"id": "2", "name": "Tijolo cerâmico 8 furos", "category": "Construção", "qty": "5000"},
	}
	sessions = map[string]bool{}
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleApp)
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/materials", handleMaterials)
	mux.HandleFunc("/api/materials/", handleMaterialByID)
	mux.HandleFunc("/api/cinematic-send", handleCinematicSend)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	addr := ":8099"
	if v := getenv("FIXTURE_ADDR"); v != "" {
		addr = v
	}
	fmt.Printf("fixture app on %s\n", addr)
	_ = http.ListenAndServe(addr, mux)
}

func getenv(k string) string {
	for _, e := range []string{k} {
		_ = e
	}
	return ""
}

func authed(r *http.Request) bool {
	c, err := r.Cookie("fixture_session")
	if err != nil {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	return sessions[c.Value]
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"hint": "POST {username,password}"})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	time.Sleep(300 * time.Millisecond)
	if body.Username == "demo" && body.Password == "demo1234" {
		mu.Lock()
		sessions["demo-session"] = true
		mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "fixture_session", Value: "demo-session", Path: "/", HttpOnly: true})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
		return
	}
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "Credenciais inválidas. Use demo / demo1234."})
}

func handleMaterials(w http.ResponseWriter, r *http.Request) {
	if !authed(r) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		time.Sleep(800 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(materials)
	case http.MethodPost:
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": "invalid json"})
			return
		}
		if strings.TrimSpace(body["name"]) == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{"error": "Nome é obrigatório", "field": "name"})
			return
		}
		mu.Lock()
		body["id"] = fmt.Sprintf("%d", len(materials)+1)
		materials = append(materials, body)
		mu.Unlock()
		time.Sleep(900 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": body["id"]})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleMaterialByID(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]any{"error": "not found"})
}

// handleCinematicSend simulates server latency for the Cinematic V2
// fixture: the wait is long enough (>1.5s) to trigger smart dead-time
// compression in the final video.
func handleCinematicSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body map[string]string
	_ = json.NewDecoder(r.Body).Decode(&body)
	time.Sleep(1800 * time.Millisecond)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": "m-1", "text": body["text"]})
}

const appHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Fixture — Materiais</title>
<style>
body{font-family:system-ui,sans-serif;margin:0;background:#f4f5f7;color:#1a1d29}
.layout{display:flex;min-height:100vh}
aside{width:220px;background:#1a1d29;color:#fff;padding:16px}
aside a{color:#fff;text-decoration:none;display:block;padding:8px;border-radius:6px}
aside a:hover,aside a.active{background:#2e3348}
main{flex:1;padding:24px}
.card{background:#fff;border-radius:10px;padding:16px;box-shadow:0 1px 3px rgba(0,0,0,.1);max-width:720px}
table{width:100%;border-collapse:collapse}
th,td{text-align:left;padding:8px;border-bottom:1px solid #eee}
button{background:#2563eb;color:#fff;border:0;border-radius:6px;padding:8px 14px;cursor:pointer}
button:disabled{opacity:.6}
input,select{padding:8px;border:1px solid #ccc;border-radius:6px;width:100%;box-sizing:border-box}
.error{color:#b91c1c;background:#fee2e2;padding:8px;border-radius:6px;display:none}
.success{color:#166534;background:#dcfce7;padding:8px;border-radius:6px;display:none}
.loading{color:#555}
dialog{border:0;border-radius:10px;padding:20px;min-width:320px}
label{display:block;margin:8px 0 4px}
</style></head>
<body>
<div class="layout">
<aside><h3>Fixture App</h3>
<nav>
<a href="/" data-testid="nav-home">Início</a>
<a href="/materiais" data-testid="nav-materiais">Materiais</a>
<a href="/login" data-testid="nav-login">Sair</a>
</nav></aside>
<main id="main"></main>
</div>
<dialog id="modal" role="dialog" aria-label="Novo material">
<h3>Novo material</h3>
<div class="error" id="formError" role="alert"></div>
<div class="success" id="formSuccess" role="status" data-testid="form-success"></div>
<label for="fName">Nome</label><input id="fName" data-testid="material-name" placeholder="Ex: Areia lavada">
<label for="fCat">Categoria</label>
<select id="fCat" data-testid="material-category"><option>Construção</option><option>Elétrica</option><option>Hidráulica</option></select>
<label for="fQty">Quantidade</label><input id="fQty" data-testid="material-qty" value="10">
<label for="fPass">Senha (demo de masking)</label><input id="fPass" type="password" data-testid="material-secret" value="">
<p><button data-testid="save-material-btn" onclick="saveMaterial()">Salvar</button>
<button onclick="document.getElementById('modal').close()">Fechar</button></p>
</dialog>
<script>
const main=document.getElementById('main');
const path=location.pathname;
document.querySelectorAll('aside a').forEach(a=>{if(a.getAttribute('href')===path)a.classList.add('active')});
if(path==='/login'){main.innerHTML='<div class=card><h1 role=heading>Entrar</h1><label for=loginUser>Usuário</label><input id=loginUser data-testid=login-user value=demo><label for=loginPass>Senha</label><input id=loginPass data-testid=login-pass type=password value=demo1234><p><button data-testid=login-btn onclick="window.doLogin()">Entrar</button></p><div class=error id=loginError></div></div>';
window.doLogin=async function(){const u=document.getElementById('loginUser').value,p=document.getElementById('loginPass').value;const r=await fetch('/api/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:u,password:p})});if(r.ok){location.href='/materiais'}else{const j=await r.json();const e=document.getElementById('loginError');e.style.display='block';e.textContent=j.error||'Falha no login'}}}
else if(path==='/materiais'){main.innerHTML='<h1 role=heading>Materiais</h1><div class=card><p><button data-testid=new-material-btn onclick=openModal()>Novo</button> <span class=loading id=loadState>Carregando...</span></p><table><thead><tr><th>Nome</th><th>Categoria</th><th>Qtd</th></tr></thead><tbody id=rows></tbody></table><div class=success id=okState></div></div>';
fetch('/api/materials').then(r=>{if(!r.ok){main.innerHTML='<div class=card><h1 role=heading>Entrar</h1><p>Sessão necessária. <a href=/login role=link>Ir para login</a></p></div>';return null}return r.json()}).then(list=>{if(!list)return;document.getElementById('loadState').textContent='';const tb=document.getElementById('rows');list.forEach(m=>{const tr=document.createElement('tr');tr.innerHTML='<td></td><td></td><td></td>';tr.children[0].textContent=m.name;tr.children[1].textContent=m.category;tr.children[2].textContent=m.qty;tb.appendChild(tr)})});
window.openModal=()=>{document.getElementById('modal').showModal()};
window.saveMaterial=async()=>{const name=document.getElementById('fName').value,category=document.getElementById('fCat').value,qty=document.getElementById('fQty').value;const err=document.getElementById('formError'),okEl=document.getElementById('formSuccess');err.style.display='none';okEl.style.display='none';const r=await fetch('/api/materials',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name,category,qty})});if(r.status===422){const j=await r.json();err.style.display='block';err.textContent=j.error;return}await new Promise(res=>setTimeout(res,300));okEl.style.display='block';okEl.textContent='Material salvo com sucesso!';setTimeout(()=>location.reload(),600)};}
else{main.innerHTML='<div class=card><h1 role=heading>Painel</h1><p>Bem-vindo ao fixture do AutoDoc. Use o menu lateral para acessar <a href=/materiais role=link>Materiais</a>.</p></div>'}
</script></body></html>`

const cinematicHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Fixture — Conversas (Cinematic V2)</title>
<style>
body{font-family:system-ui,sans-serif;margin:0;background:#f4f5f7;color:#1a1d29}
.layout{display:flex;min-height:100vh}
aside{width:230px;background:#1a1d29;color:#fff;padding:16px;flex-shrink:0}
aside h3{margin:0 0 12px}
.conv{display:block;width:100%;text-align:left;background:transparent;color:#fff;border:0;border-radius:6px;padding:10px;cursor:pointer}
.conv:hover,.conv.active{background:#2e3348}
main{flex:1;padding:24px;min-width:0}
.banner{background:linear-gradient(90deg,#2563eb,#7c3aed);color:#fff;border-radius:10px;padding:28px;margin-bottom:16px;min-height:120px}
.card{background:#fff;border-radius:10px;padding:16px;box-shadow:0 1px 3px rgba(0,0,0,.1);max-width:760px;margin-bottom:16px}
.panel{border:1px solid #e5e7eb;border-radius:8px;padding:12px;min-height:90px;margin:8px 0}
.bubble{background:#dcfce7;border-radius:8px;padding:8px 12px;margin:6px 0;display:none}
.success{color:#166534;background:#dcfce7;padding:8px;border-radius:6px;display:none}
.row{display:flex;gap:8px;margin-top:8px}
input,select{padding:8px;border:1px solid #ccc;border-radius:6px;box-sizing:border-box}
#cinematic-input{flex:1}
button{background:#2563eb;color:#fff;border:0;border-radius:6px;padding:8px 14px;cursor:pointer}
.iconbtn{background:#eef2ff;color:#1e293b;border-radius:50%;width:22px;height:22px;padding:0;font-size:12px;line-height:22px;text-align:center}
dialog{border:0;border-radius:10px;padding:20px;min-width:320px}
.loading{color:#555;display:none}
</style></head>
<body>
<div class="layout">
<aside><h3>Conversas</h3>
<nav aria-label="conversas" data-testid="conversation-list">
<button class="conv" data-testid="conv-alice" onclick="openConv('alice')">Alice Almeida</button>
<button class="conv" data-testid="conv-beto" onclick="openConv('beto')">Beto Barros</button>
</nav></aside>
<main>
<div class="banner" data-testid="cinematic-banner"><h1 style="margin:0">Central de conversas</h1><p style="margin:4px 0 0">Gerencie todas as conversas em um só lugar.</p></div>
<div class="card"><h2 style="margin-top:0">Conversa <button class="iconbtn" data-testid="cinematic-info-icon" title="info" onclick="void 0">i</button></h2>
<div class="panel" data-testid="conversation-panel" id="convPanel"><span style="color:#888">Selecione uma conversa na lateral.</span></div>
<div id="message-list" data-testid="message-list"></div>
<div class="bubble" id="bubble" data-testid="cinematic-result-bubble"></div>
<div class="success" id="okState" role="status" data-testid="cinematic-success"></div>
<div class="success" id="receipt" role="status" data-testid="cinematic-receipt" style="background:#e0e7ff;color:#3730a3"></div>
<div class="row"><input id="cinematic-input" data-testid="cinematic-input" placeholder="Digite sua mensagem"><button data-testid="cinematic-send" onclick="sendMessage()">Enviar</button></div>
<p class="loading" id="sendLoading">Enviando...</p>
</div>
<div class="card"><p><button data-testid="cinematic-modal-btn" onclick="document.getElementById('cmodal').showModal()">Abrir detalhes</button></p></div>
</main>
</div>
<dialog id="cmodal" role="dialog" aria-label="Detalhes"><h3>Detalhes da conversa</h3>
<label for="mNote">Nota</label><input id="mNote" data-testid="cinematic-modal-input" placeholder="Ex: acompanhamento">
<p><button data-testid="cinematic-modal-save" onclick="document.getElementById('cmodal').close()">Fechar</button></p></dialog>
<script>
window.openConv=function(who){document.querySelectorAll('.conv').forEach(b=>b.classList.remove('active'));var p=document.getElementById('convPanel');var name=who==='alice'?'Alice Almeida':'Beto Barros';p.innerHTML='<strong></strong> <span>conversa aberta. Histórico carregado.</span>';p.querySelector('strong').textContent=name;document.querySelector('[data-testid=conv-'+who+']').classList.add('active')};
window.sendMessage=async function(){var inp=document.getElementById('cinematic-input');var text=inp.value||'Olá, tudo bem?';var ld=document.getElementById('sendLoading');ld.style.display='block';var r=await fetch('/api/cinematic-send',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({text:text})});await r.json();ld.style.display='none';var b=document.getElementById('bubble');b.style.display='block';b.textContent=text;var ok=document.getElementById('okState');ok.style.display='block';ok.textContent='Mensagem enviada com sucesso!';setTimeout(function(){var rc=document.getElementById('receipt');rc.style.display='block';rc.textContent='Comprovante #001 · entregue às '+new Date().toLocaleTimeString();},6000);};
</script></body></html>`

func handleApp(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Write([]byte("ok"))
		return
	}
	switch r.URL.Path {
	case "/", "/materiais", "/login", "/cinematic":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/cinematic" {
			fmt.Fprint(w, cinematicHTML)
			return
		}
		fmt.Fprint(w, appHTML)
	default:
		http.NotFound(w, r)
	}
}
