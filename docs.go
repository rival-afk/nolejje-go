package main

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
)

type docsEndpoint struct {
	Path    string   `json:"path"`
	Methods []string `json:"methods"`
}

func registerDocs(public gin.IRouter, engine *gin.Engine) {
	public.GET("/docs", func(c *gin.Context) {
		routes := (*engine).Routes()

		grouped := make(map[string]map[string]bool)
		for _, route := range routes {
			if _, ok := grouped[route.Path]; !ok {
				grouped[route.Path] = make(map[string]bool)
			}
			if route.Method != http.MethodHead && route.Method != http.MethodOptions {
				grouped[route.Path][route.Method] = true
			}
		}
		delete(grouped, "/docs")

		endpoints := make([]docsEndpoint, 0, len(grouped))
		for path, methods := range grouped {
			list := make([]string, 0, len(methods))
			for method := range methods {
				list = append(list, method)
			}
			sort.Strings(list)
			endpoints = append(endpoints, docsEndpoint{Path: path, Methods: list})
		}
		sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].Path < endpoints[j].Path })

		data, err := json.Marshal(endpoints)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "Internal Server Error"})
			return
		}

		page := `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Nolejje API Docs</title>
<link rel="stylesheet" href="/css/ui.css">
<style>
.api-docs { max-width: 1000px; margin: 0 auto; padding: 20px; }
.api-docs h1 { text-align: center; }
.api-docs .method { display: inline-block; min-width: 62px; text-align: center; padding: 2px 6px; border-radius: 4px; color: #fff; font-weight: 700; font-family: monospace; font-size: 13px; }
.api-docs .GET { background: #4caf50; }
.api-docs .PATCH { background: #ff9800; }
.api-docs .POST { background: #5c67d2; }
.api-docs .DELETE { background: #f44336; }
.vault { margin: 0 auto 24px; max-width: 640px; padding: 14px; border: 1px solid rgba(128,128,128,0.25); border-radius: 10px; }
.vault .label { font-size: 13px; margin-bottom: 8px; }
.vault .hint { font-size: 12px; opacity: .65; margin-top: 6px; }
.ep { display: grid; grid-template-columns: 88px 1fr; align-items: center; gap: 12px; padding: 6px 14px; border-radius: 8px; cursor: pointer; }
.ep:nth-of-type(odd) { background: rgba(128,128,128,0.08); }
.ep a { color: inherit; text-decoration: none; font-family: monospace; }
.ep .badge { font-size: 11px; margin-left: 6px; opacity: .9; }
.ep .methods { display: grid; gap: 4px; }
.tester { display: none; padding: 14px; border: 1px solid rgba(128,128,128,0.25); border-top: 0; border-radius: 0 0 8px 8px; margin: 0 0 10px; }
.tester.open { display: block; }
.tester label { display: block; font-size: 12px; opacity: .7; margin: 10px 0 4px; }
.tester input, .tester textarea, .tester select { width: 100%; box-sizing: border-box; background: rgba(128,128,128,0.08); border: 1px solid rgba(128,128,128,0.3); color: inherit; border-radius: 6px; padding: 8px 10px; font-family: monospace; font-size: 13px; }
.tester textarea { min-height: 80px; }
.tester .methrow { display: flex; gap: 10px; align-items: center; }
.tester .methrow label { margin: 0; }
.tester .methrow select { width: 110px; }
.tester .send { margin-top: 12px; padding: 8px 18px; border: 0; border-radius: 6px; background: #5c67d2; color: #fff; font-weight: 700; cursor: pointer; }
.tester .resp { margin-top: 12px; }
.tester .resp pre { overflow: auto; background: rgba(128,128,128,0.08); padding: 10px; border-radius: 6px; font-size: 12px; max-height: 400px; margin: 0; }
.tester .resp .codewrap { font-weight: 700; margin-bottom: 6px; }
</style>
</head>
<body>
<div class="api-docs">
<h1>Nolejje API Docs</h1>

<div class="vault">
	<div class="label">🔐 Вход: токен хранится в защищённом хранилище браузера — HttpOnly-cookie (недоступна скриптам, не localStorage)</div>
	<div class="row" style="display:flex; gap:8px; margin-bottom:6px">
		<input id="vault-email" type="email" placeholder="email" autocomplete="off" style="flex:1; box-sizing:border-box; background:rgba(128,128,128,0.08); border:1px solid rgba(128,128,128,0.3); color:inherit; border-radius:6px; padding:8px 10px; font-family:monospace; font-size:13px">
		<input id="vault-pass" type="password" placeholder="пароль" style="flex:1; box-sizing:border-box; background:rgba(128,128,128,0.08); border:1px solid rgba(128,128,128,0.3); color:inherit; border-radius:6px; padding:8px 10px; font-family:monospace; font-size:13px">
		<button id="vault-login" type="button" style="border:0; border-radius:6px; padding:8px 14px; cursor:pointer; font-weight:700; color:#fff; background:#4caf50">Войти</button>
	</div>
	<div class="hint" id="vault-hint">Выполни POST /auth/login — сервер выставит HttpOnly-cookie (Secure + SameSite=Strict), и браузер будет сам подставлять её в запросы ниже. Читаемым JS токен не доступен; вместо cookie можно вписать заголовок Authorization вручную.</div>
</div>

<div id="list"></div>
</div>
<script>
var ROUTES = ` + string(data) + `;

var vaultHint = document.getElementById("vault-hint");
document.getElementById("vault-login").addEventListener("click", function () {
	var email = document.getElementById("vault-email").value.trim();
	var pass = document.getElementById("vault-pass").value;
	vaultHint.textContent = "Входим...";
	fetch("/auth/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email: email, password: pass }) })
		.then(function (r) { return r.text().then(function (t) { return { code: r.status, text: t }; }); })
		.then(function (r) {
			vaultHint.textContent = r.code === 200
				? "Готово: HttpOnly-cookie установлена, защищённые эндпоинты будут авторизованы"
				: "Ошибка входа (" + r.code + "): " + r.text;
		})
		.catch(function (e) { vaultHint.textContent = "Ошибка: " + e; });
});

var list = document.getElementById("list");
function el(tag, cls, text) { var e = document.createElement(tag); if (cls) e.className = cls; if (text !== undefined) e.textContent = text; return e; }

var NEED_BODY = { POST: true, PATCH: true, PUT: true };

ROUTES.forEach(function (ep) {
	var wrapper = el("div");

	var row = el("div", "ep");
	var methods = el("div", "methods");
	ep.methods.forEach(function (m) { methods.appendChild(el("span", "method " + m, m)); });
	row.appendChild(methods);
	var linkWrap = el("div");
	linkWrap.appendChild(el("a", null, ep.path));
	ep.methods.slice(1).forEach(function (m) { linkWrap.appendChild(el("span", "method badge " + m, m)); });
	row.appendChild(linkWrap);
	wrapper.appendChild(row);

	var form = document.createElement("form");
	var tester = el("div", "tester");
	form.appendChild(tester);

	tester.addEventListener("click", function (ev) { ev.stopPropagation(); });
	row.addEventListener("click", function () { tester.classList.toggle("open"); });

	var methrow = el("div", "methrow");
	methrow.appendChild(el("label", null, "Метод"));
	var methodSel = document.createElement("select");
	ep.methods.forEach(function (m) { var o = document.createElement("option"); o.value = m; o.textContent = m; methodSel.appendChild(o); });
	methrow.appendChild(methodSel);
	tester.appendChild(methrow);

	var pathParams = {};
	(ep.path.match(/:[^\/]+/g) || []).forEach(function (name) {
		var key = name.slice(1);
		tester.appendChild(el("label", null, "Параметр пути " + key));
		var inp = document.createElement("input");
		inp.placeholder = "1"; inp.setAttribute("autocomplete", "off");
		pathParams[key] = inp;
		tester.appendChild(inp);
	});

	tester.appendChild(el("label", null, "Query-параметры (key=value, по одной в строке)"));
	var queryInput = document.createElement("textarea");
	queryInput.rows = 2;
	tester.appendChild(queryInput);

	var bodyLabel = el("label", null, "Тело запроса (JSON)");
	var bodyInput = document.createElement("textarea");
	bodyInput.value = "{\\n}"; bodyInput.rows = 4;
	function syncBodyVisibility() {
		var need = NEED_BODY[methodSel.value];
		bodyLabel.style.display = need ? "" : "none";
		bodyInput.style.display = need ? "" : "none";
	}
	tester.appendChild(bodyLabel);
	tester.appendChild(bodyInput);
	methodSel.addEventListener("change", syncBodyVisibility);
	syncBodyVisibility();

	tester.appendChild(el("label", null, "Заголовки (необязательно — HttpOnly-cookie с токеном браузер отправит сам; можно задать вручную, key=value по строкам)"));
	var headersInput = document.createElement("textarea");
	headersInput.className = "hdr";
	headersInput.rows = 2;
	tester.appendChild(headersInput);

	var sendBtn = document.createElement("button");
	sendBtn.type = "submit"; sendBtn.className = "send"; sendBtn.textContent = "Отправить";
	tester.appendChild(sendBtn);

	var resp = el("div", "resp");
	tester.appendChild(resp);

	form.onsubmit = function (ev) {
		ev.preventDefault();
		resp.textContent = "";
		var url = ep.path;
		Object.keys(pathParams).forEach(function (k) {
			url = url.split(":" + k).join(encodeURIComponent(pathParams[k].value.trim()));
		});
		var qs = queryInput.value.split("\\n").map(function (l) { return l.trim(); }).filter(function (l) { return l; });
		if (qs.length) url += "?" + qs.join("&");

		var headers = {};
		headersInput.value.split("\\n").map(function (l) { return l.trim(); }).filter(function (l) { return l; }).forEach(function (l) {
			var i = l.indexOf("=");
			if (i > 0) headers[l.slice(0, i).trim()] = l.slice(i + 1).trim();
		});

		var init = { method: methodSel.value, headers: headers };
		if (NEED_BODY[methodSel.value]) {
			init.body = bodyInput.value;
			headers["Content-Type"] = "application/json";
		}

		var t0 = performance.now();
		fetch(url, init).then(function (r) {
			var ms = Math.round(performance.now() - t0);
			resp.appendChild(el("div", "codewrap", "Статус " + r.status + " (" + r.statusText + ") — " + ms + " мс"));
			return r.text().then(function (txt) {
				var pretty = txt;
				try { pretty = JSON.stringify(JSON.parse(txt), null, 2); } catch (e) {}
				var pre = document.createElement("pre");
				pre.textContent = pretty || "(пусто)";
				resp.appendChild(pre);
			});
		}).catch(function (e) {
			resp.appendChild(el("div", "codewrap", "Ошибка: " + e));
		});
		return false;
	};

	wrapper.appendChild(form);
	list.appendChild(wrapper);
});
</script>
</body>
</html>`

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
	})
}
