/* ============================================================
   Shorty — frontend logic
   Talks to the Go backend: POST /url/shorten, GET /api/urls/recent
   ============================================================ */

(() => {
    "use strict";

    const form = document.getElementById("shorten-form");
    const input = document.getElementById("url-input");
    const clearBtn = document.getElementById("clear-btn");
    const shortenBtn = document.getElementById("shorten-btn");
    const btnLabel = shortenBtn.querySelector(".btn-label");
    const chopIcon = shortenBtn.querySelector(".chop-icon");
    const errorMsg = document.getElementById("error-msg");

    const resultCard = document.getElementById("result-card");
    const resultUrl = document.getElementById("result-url");
    const copyBtn = document.getElementById("copy-btn");
    const visitBtn = document.getElementById("visit-btn");
    const qrToggle = document.getElementById("qr-toggle");
    const qrPanel = document.getElementById("qr-panel");
    const qrImage = document.getElementById("qr-image");

    const recentSection = document.getElementById("recent-section");
    const recentList = document.getElementById("recent-list");
    const recentEmpty = document.getElementById("recent-empty");
    const skeleton = document.getElementById("skeleton");
    const refreshBtn = document.getElementById("refresh-btn");

    const toast = document.getElementById("toast");
    const yearEl = document.getElementById("year");

    let toastTimer = null;

    yearEl.textContent = new Date().getFullYear();

    /* ---------------- Helpers ---------------- */

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

    function setLoading(isLoading) {
        shortenBtn.disabled = isLoading;
        shortenBtn.classList.toggle("is-loading", isLoading);
        btnLabel.textContent = isLoading ? "Cutting…" : "Chop it";
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

    function clearError() {
        errorMsg.textContent = "";
        errorMsg.hidden = true;
        input.classList.remove("invalid");
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

    /* ---------------- Shorten flow ---------------- */

    async function shorten(urlValue) {
        clearError();
        setLoading(true);

        try {
            const res = await fetch("/url/shorten", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ url: urlValue }),
            });

            const data = await res.json().catch(() => ({}));

            if (!res.ok || !data.success) {
                showError(data.error || "that didn't work — try again.", true);
                return;
            }

            resultUrl.textContent = data.shortUrl;
            visitBtn.href = data.shortUrl;
            resetCopyButton();
            qrPanel.hidden = true;
            qrToggle.setAttribute("aria-expanded", "false");
            qrToggle.textContent = "show QR";
            resultCard.hidden = false;

            loadRecent();
        } catch {
            showError("network error — is the server running?", true);
        } finally {
            setLoading(false);
        }
    }

    function resetCopyButton() {
        copyBtn.classList.remove("copied");
        copyBtn.querySelector(".copy-label").textContent = "copy link";
    }

    /* ---------------- Copy / QR ---------------- */

    copyBtn.addEventListener("click", () => {
        const url = resultUrl.textContent;
        if (url) copyText(url, copyBtn, ".copy-label", "shorty link copied");
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

    /* ---------------- Input wiring ---------------- */

    input.addEventListener("input", () => {
        clearError();
        clearBtn.hidden = input.value.length === 0;
    });

    clearBtn.addEventListener("click", () => {
        input.value = "";
        clearError();
        clearBtn.hidden = true;
        input.focus();
        resetState();
    });

    form.addEventListener("submit", (event) => {
        event.preventDefault();

        const value = input.value.trim();

        if (!value) {
            showError("paste a URL first.", true);
            input.focus();
            return;
        }

        if (!isValidUrl(value)) {
            showError("that doesn't look like a URL.", true);
            input.focus();
            return;
        }

        shorten(value);
    });

    document.addEventListener("keydown", (event) => {
        if (event.key === "Escape") {
            if (!resultCard.hidden && document.activeElement !== input) {
                resetState();
                return;
            }
            input.value = "";
            clearError();
            clearBtn.hidden = true;
            input.focus();
        }
    });

    function resetState() {
        resultCard.hidden = true;
        resetCopyButton();
        qrPanel.hidden = true;
        qrToggle.textContent = "show QR";
        qrToggle.setAttribute("aria-expanded", "false");
    }

    /* ---------------- Recent links ---------------- */

    async function loadRecent() {
        showSkeleton(true);

        try {
            const res = await fetch("/api/urls/recent");
            const data = await res.json().catch(() => ({}));

            showSkeleton(false);

            if (!res.ok || !data.success) {
                recentSection.hidden = true;
                return;
            }

            const items = (data.data || []).slice(0, 2);

            if (items.length === 0) {
                recentSection.hidden = false;
                recentList.replaceChildren();
                recentEmpty.hidden = false;
                return;
            }

            recentSection.hidden = false;
            recentEmpty.hidden = true;
            renderRecent(items);
        } catch {
            showSkeleton(false);
            recentSection.hidden = true;
        }
    }

    function showSkeleton(isVisible) {
        skeleton.hidden = !isVisible;
    }

    function renderRecent(items) {
        const fragment = document.createDocumentFragment();

        items.forEach((item, index) => {
            const li = document.createElement("li");
            li.className = "rec-row";
            li.style.animationDelay = Math.min(index * 55, 275) + "ms";

            const code = document.createElement("span");
            code.className = "rec-code";
            code.textContent = item.shortCode;

            const meta = document.createElement("div");
            meta.className = "rec-meta";

            const original = document.createElement("span");
            original.className = "rec-url";
            original.textContent = item.originalUrl;

            const time = document.createElement("span");
            time.className = "rec-time";
            time.textContent = formatTime(item.createdAt);

            meta.append(original, time);

            const copy = document.createElement("button");
            copy.type = "button";
            copy.className = "rec-copy";
            copy.textContent = "copy";
            copy.setAttribute("aria-label", "Copy " + item.shortUrl);
            copy.addEventListener("click", () => copyText(item.shortUrl, copy));

            li.append(code, meta, copy);
            fragment.append(li);
        });

        recentList.replaceChildren(fragment);
    }

    function formatTime(value) {
        if (!value) return "";
        const date = new Date(value.replace(" ", "T") + "Z");
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
            year: date.getFullYear() !== new Date().getFullYear() ? "numeric" : undefined,
        });
    }

    refreshBtn.addEventListener("click", loadRecent);

    /* ---------------- Init ---------------- */

    loadRecent();
})();