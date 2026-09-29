/* ============================================================
   Shorty v2 — frontend logic
   Full integration with Go backend:
   - PostgreSQL & Redis backing
   - JWT Auth (register, login, refresh, logout)
   - Custom aliases & 2h ephemeral link badges
   - Real-time click analytics & daily charts
   - User dashboard (My Saved Links)
   ============================================================ */

(() => {
    "use strict";

    // Auth state
    let currentUser = null;
    let accessToken = localStorage.getItem("shorty_token") || null;
    let activeDashboardTab = "recent"; // "recent" | "mylinks"
    let currentResultCode = null;

    // DOM Elements - Form
    const form = document.getElementById("shorten-form");
    const input = document.getElementById("url-input");
    const clearBtn = document.getElementById("clear-btn");
    const shortenBtn = document.getElementById("shorten-btn");
    const btnLabel = shortenBtn.querySelector(".btn-label");
    const errorMsg = document.getElementById("error-msg");

    // Custom Alias Elements
    const aliasToggleBtn = document.getElementById("alias-toggle-btn");
    const aliasWrap = document.getElementById("alias-wrap");
    const aliasInput = document.getElementById("alias-input");
    const anonNotice = document.getElementById("anon-notice");

    // Result Card Elements
    const resultCard = document.getElementById("result-card");
    const resultUrl = document.getElementById("result-url");
    const resultBadge = document.getElementById("result-badge");
    const resultExpiryBanner = document.getElementById("result-expiry-banner");
    const copyBtn = document.getElementById("copy-btn");
    const visitBtn = document.getElementById("visit-btn");
    const analyticsBtn = document.getElementById("analytics-btn");
    const qrToggle = document.getElementById("qr-toggle");
    const qrPanel = document.getElementById("qr-panel");
    const qrImage = document.getElementById("qr-image");

    // Dashboard Elements
    const tabRecent = document.getElementById("tab-recent");
    const tabMyLinks = document.getElementById("tab-mylinks");
    const myLinksCount = document.getElementById("my-links-count");
    const linkList = document.getElementById("link-list");
    const listEmpty = document.getElementById("list-empty");
    const skeleton = document.getElementById("skeleton");
    const refreshBtn = document.getElementById("refresh-btn");

    // Auth Bar Elements
    const authGuest = document.getElementById("auth-guest");
    const authUser = document.getElementById("auth-user");
    const userDisplayName = document.getElementById("user-display-name");
    const loginOpenBtn = document.getElementById("login-open-btn");
    const registerOpenBtn = document.getElementById("register-open-btn");
    const logoutBtn = document.getElementById("logout-btn");

    // Auth Modal Elements
    const authModal = document.getElementById("auth-modal");
    const authCloseBtn = document.getElementById("auth-close-btn");
    const authTabLogin = document.getElementById("auth-tab-login");
    const authTabRegister = document.getElementById("auth-tab-register");
    const authForm = document.getElementById("auth-form");
    const authNameGroup = document.getElementById("auth-name-group");
    const authName = document.getElementById("auth-name");
    const authEmail = document.getElementById("auth-email");
    const authPassword = document.getElementById("auth-password");
    const authSubmitLabel = document.getElementById("auth-submit-label");
    const authErrorMsg = document.getElementById("auth-error-msg");
    let isRegisterMode = false;

    // Analytics Modal Elements
    const analyticsModal = document.getElementById("analytics-modal");
    const statsCloseBtn = document.getElementById("stats-close-btn");
    const statsCode = document.getElementById("stats-code");
    const kpiClicks = document.getElementById("kpi-clicks");
    const kpiVisitors = document.getElementById("kpi-visitors");
    const kpiType = document.getElementById("kpi-type");
    const chartBars = document.getElementById("chart-bars");
    const referersList = document.getElementById("referers-list");
    const devicesList = document.getElementById("devices-list");

    const toast = document.getElementById("toast");
    const yearEl = document.getElementById("year");
    let toastTimer = null;

    if (yearEl) yearEl.textContent = new Date().getFullYear();

    /* ---------------- Auth Helpers ---------------- */

    function getAuthHeaders() {
        const headers = { "Content-Type": "application/json" };
        if (accessToken) {
            headers["Authorization"] = "Bearer " + accessToken;
        }
        return headers;
    }

    function setAuthState(token, user) {
        accessToken = token;
        currentUser = user;
        if (token) {
            localStorage.setItem("shorty_token", token);
            if (user) localStorage.setItem("shorty_user", JSON.stringify(user));
        } else {
            localStorage.removeItem("shorty_token");
            localStorage.removeItem("shorty_user");
        }
        renderAuthUI();
    }

    function renderAuthUI() {
        if (accessToken && currentUser) {
            authGuest.hidden = true;
            authUser.hidden = false;
            userDisplayName.textContent = "👤 " + (currentUser.displayName || currentUser.email.split("@")[0]);
            tabMyLinks.hidden = false;
            anonNotice.textContent = "✓ Permanent links";
            anonNotice.style.background = "#e2f0d9";
            anonNotice.style.color = "#276738";
        } else {
            authGuest.hidden = false;
            authUser.hidden = true;
            tabMyLinks.hidden = true;
            if (activeDashboardTab === "mylinks") {
                switchDashboardTab("recent");
            }
            anonNotice.textContent = "⚡ Anon links: 2h expiry";
            anonNotice.style.background = "";
            anonNotice.style.color = "";
        }
    }

    async function checkAuthSession() {
        const savedUser = localStorage.getItem("shorty_user");
        if (savedUser) {
            try { currentUser = JSON.parse(savedUser); } catch {}
        }

        if (accessToken) {
            try {
                const res = await fetch("/api/user/me", { headers: getAuthHeaders() });
                if (res.ok) {
                    const data = await res.json();
                    setAuthState(accessToken, data.user);
                    return;
                }
            } catch {}
        }

        // Try silent refresh via cookie
        try {
            const res = await fetch("/api/auth/refresh", { method: "POST" });
            if (res.ok) {
                const data = await res.json();
                const meRes = await fetch("/api/user/me", {
                    headers: { "Authorization": "Bearer " + data.accessToken }
                });
                if (meRes.ok) {
                    const meData = await meRes.json();
                    setAuthState(data.accessToken, meData.user);
                    return;
                }
            }
        } catch {}

        setAuthState(null, null);
    }

    /* ---------------- Utility Helpers ---------------- */

    function isValidUrl(value) {
        try {
            const original = value.trim();
            const candidate = /^https?:\/\//i.test(original) ? original : "https://" + original;
            const url = new URL(candidate);
            return url.protocol === "http:" || url.protocol === "https:";
        } catch {
            return false;
        }
    }

    function showToast(message, type) {
        toast.textContent = message;
        toast.className = "toast" + (type ? " " + type : "");
        void toast.offsetWidth;
        toast.classList.add("show");
        clearTimeout(toastTimer);
        toastTimer = setTimeout(() => toast.classList.remove("show"), 2600);
    }

    function setCopied(button, labelSelector) {
        button.classList.add("copied");
        if (labelSelector) {
            const label = button.querySelector(labelSelector);
            if (label) label.textContent = "copied!";
        }
        setTimeout(() => {
            button.classList.remove("copied");
            if (labelSelector) {
                const label = button.querySelector(labelSelector);
                if (label) label.textContent = "copy link";
            }
        }, 1800);
    }

    async function copyText(text, button, labelSelector, toastMsg) {
        try {
            await navigator.clipboard.writeText(text);
            setCopied(button, labelSelector);
            showToast(toastMsg || "copied to clipboard");
        } catch {
            showToast("copy failed — pick the link manually", "error");
        }
    }

    function formatTime(value) {
        if (!value) return "";
        const date = new Date(value);
        if (isNaN(date.getTime())) return value;

        const diff = Date.now() - date.getTime();
        const minutes = Math.floor(diff / 60000);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        if (minutes < 1) return "just now";
        if (minutes < 60) return minutes + "m ago";
        if (hours < 24) return hours + "h ago";
        if (days < 7) return days + "d ago";

        return date.toLocaleDateString(undefined, {
            month: "short",
            day: "numeric",
        });
    }

    /* ---------------- Shorten Flow ---------------- */

    function clearError() {
        errorMsg.textContent = "";
        errorMsg.hidden = true;
        input.classList.remove("invalid");
    }

    function showError(message, shake) {
        errorMsg.textContent = message;
        errorMsg.hidden = false;
        input.classList.add("invalid");
        if (shake) {
            errorMsg.classList.remove("shake");
            void errorMsg.offsetWidth;
            errorMsg.classList.add("shake");
        }
    }

    function setLoading(isLoading) {
        shortenBtn.disabled = isLoading;
        shortenBtn.classList.toggle("is-loading", isLoading);
        btnLabel.textContent = isLoading ? "Cutting…" : "Chop it";
    }

    aliasToggleBtn.addEventListener("click", () => {
        const isOpen = aliasWrap.hidden === false;
        aliasWrap.hidden = isOpen;
        aliasToggleBtn.setAttribute("aria-expanded", String(!isOpen));
        aliasToggleBtn.textContent = isOpen ? "+ custom alias" : "– hide alias";
        if (!isOpen && !accessToken) {
            showToast("Tip: Log in to save custom aliases permanently!", "info");
        }
    });

    async function shorten(urlValue, customCode) {
        clearError();
        setLoading(true);

        const payload = { url: urlValue };
        if (customCode) payload.customCode = customCode;

        try {
            const res = await fetch("/url/shorten", {
                method: "POST",
                headers: getAuthHeaders(),
                body: JSON.stringify(payload),
            });

            const data = await res.json().catch(() => ({}));

            if (!res.ok || !data.success) {
                showError(data.error || "That didn't work — try again.", true);
                if (data.error && data.error.includes("account")) {
                    openAuthModal(false);
                }
                return;
            }

            currentResultCode = data.shortCode;
            resultUrl.textContent = data.shortUrl;
            visitBtn.href = data.shortUrl;

            // Render status badge & banner
            if (data.isEphemeral) {
                resultBadge.textContent = "⏱ 2h link";
                resultBadge.className = "badge-status badge-ephemeral mono";
                resultExpiryBanner.textContent = "⏱ This anonymous link vanishes in 2 hours. Log in to keep links forever.";
                resultExpiryBanner.hidden = false;
            } else {
                resultBadge.textContent = "✓ Permanent";
                resultBadge.className = "badge-status badge-permanent mono";
                resultExpiryBanner.hidden = true;
            }

            resetCopyButton();
            qrPanel.hidden = true;
            qrToggle.setAttribute("aria-expanded", "false");
            qrToggle.textContent = "show QR";
            resultCard.hidden = false;

            // Reload active tab
            loadDashboard();
        } catch {
            showError("Network error — is the server running?", true);
        } finally {
            setLoading(false);
        }
    }

    function resetCopyButton() {
        copyBtn.classList.remove("copied");
        copyBtn.querySelector(".copy-label").textContent = "copy link";
    }

    copyBtn.addEventListener("click", () => {
        const url = resultUrl.textContent;
        if (url) copyText(url, copyBtn, ".copy-label", "Shorty link copied");
    });

    qrToggle.addEventListener("click", () => {
        const isOpen = qrPanel.hidden === false;
        qrPanel.hidden = isOpen;
        qrToggle.setAttribute("aria-expanded", String(!isOpen));
        qrToggle.textContent = isOpen ? "show QR" : "hide QR";

        if (!isOpen) {
            const url = resultUrl.textContent;
            qrImage.src =
                "https://api.qrserver.com/v1/create-qr-code/?size=160x160&qzone=1&data=" +
                encodeURIComponent(url);
            showToast("scan to share on mobile");
        }
    });

    analyticsBtn.addEventListener("click", () => {
        if (currentResultCode) {
            openAnalytics(currentResultCode);
        }
    });

    input.addEventListener("input", () => {
        clearError();
        clearBtn.hidden = input.value.length === 0;
    });

    clearBtn.addEventListener("click", () => {
        input.value = "";
        clearError();
        clearBtn.hidden = true;
        input.focus();
    });

    form.addEventListener("submit", (e) => {
        e.preventDefault();
        const value = input.value.trim();
        const customCode = aliasInput.value.trim();

        if (!value) {
            showError("Paste a URL first.", true);
            input.focus();
            return;
        }

        if (!isValidUrl(value)) {
            showError("That doesn't look like a valid URL.", true);
            input.focus();
            return;
        }

        shorten(value, customCode);
    });

    /* ---------------- Dashboard & Links ---------------- */

    function switchDashboardTab(tabName) {
        activeDashboardTab = tabName;
        tabRecent.classList.toggle("active", tabName === "recent");
        tabMyLinks.classList.toggle("active", tabName === "mylinks");
        loadDashboard();
    }

    tabRecent.addEventListener("click", () => switchDashboardTab("recent"));
    tabMyLinks.addEventListener("click", () => switchDashboardTab("mylinks"));
    refreshBtn.addEventListener("click", () => loadDashboard());

    async function loadDashboard() {
        skeleton.hidden = false;
        listEmpty.hidden = true;

        const isMyLinks = activeDashboardTab === "mylinks" && accessToken;
        const url = isMyLinks ? "/api/user/urls" : "/api/urls/recent";

        try {
            const res = await fetch(url, { headers: getAuthHeaders() });
            const data = await res.json().catch(() => ({}));
            skeleton.hidden = true;

            if (!res.ok || !data.success) {
                linkList.replaceChildren();
                listEmpty.hidden = false;
                return;
            }

            const items = data.data || [];
            if (isMyLinks) {
                myLinksCount.textContent = items.length;
            }

            if (items.length === 0) {
                linkList.replaceChildren();
                listEmpty.textContent = isMyLinks ? "You have no saved links yet." : "No shorts yet — paste a link above.";
                listEmpty.hidden = false;
                return;
            }

            renderLinkList(items, isMyLinks);
        } catch {
            skeleton.hidden = true;
            listEmpty.textContent = "Error loading links.";
            listEmpty.hidden = false;
        }
    }

    function renderLinkList(items, isOwnerView) {
        const fragment = document.createDocumentFragment();

        items.forEach((item, index) => {
            const li = document.createElement("li");
            li.className = "rec-row";
            li.style.animationDelay = Math.min(index * 50, 300) + "ms";

            // Code element
            const code = document.createElement("span");
            code.className = "rec-code";
            code.textContent = item.shortCode;

            // Meta info
            const meta = document.createElement("div");
            meta.className = "rec-meta";

            const original = document.createElement("span");
            original.className = "rec-url";
            original.textContent = item.originalUrl;

            const time = document.createElement("span");
            time.className = "rec-time";
            const timeText = formatTime(item.createdAt);
            time.textContent = item.isEphemeral ? `⏱ anon · ${timeText}` : `✓ saved · ${timeText}`;

            meta.append(original, time);

            // Click count badge
            const clickPill = document.createElement("button");
            clickPill.type = "button";
            clickPill.className = "rec-clicks mono";
            clickPill.innerHTML = `👁 ${item.clicksCount || 0}`;
            clickPill.title = "View analytics for " + item.shortCode;
            clickPill.addEventListener("click", () => openAnalytics(item.shortCode));

            // Actions group
            const actions = document.createElement("div");
            actions.className = "rec-actions";

            const copy = document.createElement("button");
            copy.type = "button";
            copy.className = "rec-copy";
            copy.textContent = "copy";
            copy.setAttribute("aria-label", "Copy " + item.shortUrl);
            copy.addEventListener("click", () => copyText(item.shortUrl, copy));

            actions.append(copy);

            if (isOwnerView) {
                const del = document.createElement("button");
                del.type = "button";
                del.className = "rec-copy btn-del";
                del.textContent = "del";
                del.title = "Delete this link";
                del.addEventListener("click", () => deleteLink(item.shortCode));
                actions.append(del);
            }

            li.append(code, meta, clickPill, actions);
            fragment.append(li);
        });

        linkList.replaceChildren(fragment);
    }

    async function deleteLink(code) {
        if (!confirm(`Are you sure you want to delete shorty/${code}?`)) return;

        try {
            const res = await fetch(`/api/user/urls/${encodeURIComponent(code)}`, {
                method: "DELETE",
                headers: getAuthHeaders(),
            });
            if (res.ok) {
                showToast(`shorty/${code} deleted`);
                loadDashboard();
            } else {
                showToast("Failed to delete link", "error");
            }
        } catch {
            showToast("Network error while deleting link", "error");
        }
    }

    /* ---------------- Analytics Modal ---------------- */

    async function openAnalytics(code) {
        statsCode.textContent = "shorty/" + code;
        analyticsModal.hidden = false;

        kpiClicks.textContent = "…";
        kpiVisitors.textContent = "…";
        kpiType.textContent = "…";
        chartBars.innerHTML = '<span class="mono" style="font-size:0.75rem;color:var(--ink-faint)">Loading insights…</span>';
        referersList.innerHTML = "";
        devicesList.innerHTML = "";

        try {
            const res = await fetch(`/api/urls/${encodeURIComponent(code)}/stats`);
            const json = await res.json().catch(() => ({}));

            if (!res.ok || !json.success) {
                showToast(json.error || "Failed to load analytics", "error");
                analyticsModal.hidden = true;
                return;
            }

            const data = json.data;
            kpiClicks.textContent = data.totalClicks || 0;
            kpiVisitors.textContent = data.uniqueVisitors || 0;
            kpiType.textContent = data.isEphemeral ? "2h Anon" : "Permanent";

            // Render 14-day chart
            renderChartBars(data.clicksByDay || []);

            // Render Referrers
            renderList(referersList, data.topReferers, "referer", "clicks");

            // Render Devices
            renderList(devicesList, data.deviceBreakdown, "device", "clicks");
        } catch {
            showToast("Error loading analytics data", "error");
        }
    }

    function renderChartBars(days) {
        chartBars.innerHTML = "";
        if (days.length === 0) {
            chartBars.innerHTML = '<span class="mono" style="font-size:0.75rem;color:var(--ink-faint);margin:auto">No click activity recorded yet.</span>';
            return;
        }

        const maxClicks = Math.max(...days.map(d => d.clicks), 1);

        // Reverse so chronological order (left to right)
        const sorted = [...days].reverse();

        sorted.forEach(d => {
            const col = document.createElement("div");
            col.className = "chart-bar-col";

            const pct = Math.max((d.clicks / maxClicks) * 100, 8);

            const fill = document.createElement("div");
            fill.className = "chart-bar-fill";
            fill.style.height = pct + "%";
            fill.title = `${d.date}: ${d.clicks} clicks`;

            const label = document.createElement("span");
            label.className = "chart-bar-label mono";
            label.textContent = d.date.slice(5); // MM-DD

            col.append(fill, label);
            chartBars.append(col);
        });
    }

    function renderList(container, items, nameKey, countKey) {
        container.innerHTML = "";
        if (!items || items.length === 0) {
            container.innerHTML = '<li class="mono" style="font-size:0.75rem;color:var(--ink-faint)">No data yet</li>';
            return;
        }

        items.forEach(it => {
            const li = document.createElement("li");
            li.className = "stats-breakdown-item";

            const name = document.createElement("span");
            name.className = "name";
            name.textContent = it[nameKey] || "unknown";

            const count = document.createElement("span");
            count.className = "count";
            count.textContent = it[countKey] || 0;

            li.append(name, count);
            container.append(li);
        });
    }

    statsCloseBtn.addEventListener("click", () => {
        analyticsModal.hidden = true;
    });

    /* ---------------- Auth Modal ---------------- */

    function openAuthModal(registerMode) {
        isRegisterMode = registerMode;
        authTabLogin.classList.toggle("active", !registerMode);
        authTabRegister.classList.toggle("active", registerMode);
        authNameGroup.hidden = !registerMode;
        authSubmitLabel.textContent = registerMode ? "Sign up" : "Log in";
        authErrorMsg.hidden = true;
        authErrorMsg.textContent = "";
        authModal.hidden = false;
        authEmail.focus();
    }

    loginOpenBtn.addEventListener("click", () => openAuthModal(false));
    registerOpenBtn.addEventListener("click", () => openAuthModal(true));
    authCloseBtn.addEventListener("click", () => { authModal.hidden = true; });

    authTabLogin.addEventListener("click", () => openAuthModal(false));
    authTabRegister.addEventListener("click", () => openAuthModal(true));

    logoutBtn.addEventListener("click", async () => {
        try {
            await fetch("/api/auth/logout", { method: "POST" });
        } catch {}
        setAuthState(null, null);
        showToast("Logged out successfully");
        loadDashboard();
    });

    authForm.addEventListener("submit", async (e) => {
        e.preventDefault();
        authErrorMsg.hidden = true;

        const email = authEmail.value.trim();
        const password = authPassword.value;
        const displayName = authName.value.trim();

        if (!email || !password) {
            authErrorMsg.textContent = "Please fill in all required fields.";
            authErrorMsg.hidden = false;
            return;
        }

        const endpoint = isRegisterMode ? "/api/auth/register" : "/api/auth/login";
        const payload = { email, password };
        if (isRegisterMode && displayName) payload.displayName = displayName;

        try {
            const res = await fetch(endpoint, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(payload),
            });

            const data = await res.json().catch(() => ({}));

            if (!res.ok || !data.success) {
                authErrorMsg.textContent = data.error || "Authentication failed.";
                authErrorMsg.hidden = false;
                return;
            }

            setAuthState(data.accessToken, data.user);
            authModal.hidden = true;
            authForm.reset();
            showToast(isRegisterMode ? "Account created! Welcome to Shorty." : "Welcome back!");
            loadDashboard();
        } catch {
            authErrorMsg.textContent = "Network error. Please try again.";
            authErrorMsg.hidden = false;
        }
    });

    // Close modals on Escape key or backdrop click
    document.addEventListener("keydown", (e) => {
        if (e.key === "Escape") {
            if (!authModal.hidden) authModal.hidden = true;
            if (!analyticsModal.hidden) analyticsModal.hidden = true;
        }
    });

    authModal.addEventListener("click", (e) => {
        if (e.target === authModal) authModal.hidden = true;
    });

    analyticsModal.addEventListener("click", (e) => {
        if (e.target === analyticsModal) analyticsModal.hidden = true;
    });

    /* ---------------- Init ---------------- */

    (async () => {
        await checkAuthSession();
        loadDashboard();
    })();
})();