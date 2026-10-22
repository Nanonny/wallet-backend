    (function () {
      "use strict";

      var TOKEN_KEY = "apex_token";
      var USERNAME_KEY = "apex_username";

      var $ = function (id) { return document.getElementById(id); };
      var state = "login";
      var busy = false;

      // If a valid session already exists, go straight to the console.
      if (window.localStorage.getItem(TOKEN_KEY)) {
        window.location.replace("/");
        return;
      }

      function notice(message, kind) {
        var el = $("notice");
        el.textContent = message;
        el.className = "notice show " + kind;
      }

      function setBusy(on) {
        busy = on;
        $("btn-login").disabled = on;
        $("btn-register").disabled = on;
      }

      function setMode(next) {
        state = next;
        var isLogin = next === "login";
        $("tab-login").classList.toggle("active", isLogin);
        $("tab-register").classList.toggle("active", !isLogin);
        $("tab-login").setAttribute("aria-selected", String(isLogin));
        $("tab-register").setAttribute("aria-selected", String(!isLogin));
        $("form-login").hidden = !isLogin;
        $("form-register").hidden = isLogin;
        $("card-title").textContent = isLogin ? "Sign in to your wallet" : "Create your account";
        $("card-sub").textContent = isLogin
          ? "Enter your account to continue to the console."
          : "One username and one password is all it takes.";
        $("notice").className = "notice";
        $("notice").textContent = "";
      }

      $("tab-login").addEventListener("click", function () { setMode("login"); });
      $("tab-register").addEventListener("click", function () { setMode("register"); });

      Array.prototype.forEach.call(document.querySelectorAll(".toggle-pw"), function (btn) {
        btn.addEventListener("click", function () {
          var input = $(btn.getAttribute("data-target"));
          var show = input.type === "password";
          input.type = show ? "text" : "password";
          btn.textContent = show ? "Hide" : "Show";
          btn.setAttribute("aria-label", (show ? "Hide" : "Show") + " password");
        });
      });

      function postJSON(url, body) {
        return fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
      }

      function finishLogin(token, username) {
        window.localStorage.setItem(TOKEN_KEY, token);
        window.localStorage.setItem(USERNAME_KEY, username);
        window.location.replace("/");
      }

      $("form-login").addEventListener("submit", function (event) {
        event.preventDefault();
        if (busy) return;
        setBusy(true);
        postJSON("/v1/auth/login", {
          username: $("login-username").value,
          password: $("login-password").value,
        })
          .then(function (res) {
            return res.json().catch(function () { return {}; }).then(function (data) {
              return { ok: res.ok, status: res.status, data: data };
            });
          })
          .then(function (out) {
            if (out.ok) {
              finishLogin(out.data.token, out.data.user.username);
              return;
            }
            if (out.status === 429) {
              notice(out.data.error || "Too many attempts. Please wait a moment.", "error");
            } else {
              notice(out.data.error || "Sign in failed. Check your username and password.", "error");
            }
          })
          .catch(function () {
            notice("Could not reach the wallet server. Is wallet-api running?", "error");
          })
          .finally(function () { setBusy(false); });
      });

      var USERNAME_OK = /^[a-z0-9_]{3,32}$/;
      var PASSWORD_OK = function (value) {
        return value.length >= 8 && value.length <= 128 && /[a-zA-Z]/.test(value) && /[0-9]/.test(value);
      };

      $("form-register").addEventListener("submit", function (event) {
        event.preventDefault();
        if (busy) return;

        var username = $("reg-username").value;
        var password = $("reg-password").value;
        var confirm = $("reg-confirm").value;

        if (!USERNAME_OK.test(username)) {
          notice("Username must be 3–32 characters: lowercase letters, numbers, or underscore.", "error");
          return;
        }
        if (!PASSWORD_OK(password)) {
          notice("Password must be at least 8 characters with at least one letter and one number.", "error");
          return;
        }
        if (password !== confirm) {
          notice("Passwords do not match.", "error");
          return;
        }

        setBusy(true);
        postJSON("/v1/auth/register", { username: username, password: password })
          .then(function (res) {
            return res.json().catch(function () { return {}; }).then(function (data) {
              return { ok: res.ok, status: res.status, data: data };
            });
          })
          .then(function (out) {
            if (!out.ok) {
              notice(out.data.error || "Registration failed.", "error");
              return;
            }
            // Switch to the login card with the new username prefilled.
            $("login-username").value = username;
            $("login-password").value = password;
            $("login-password").focus();
            setMode("login");
            notice("Account created — sign in to continue.", "info");
          })
          .catch(function () {
            notice("Could not reach the wallet server. Is wallet-api running?", "error");
          })
          .finally(function () { setBusy(false); });
      });

      setMode("login");
      $("login-username").focus();
    })();
