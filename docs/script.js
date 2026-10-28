/* ==========================================================================
   LagerKV — page controller
   Architecture tabs · mobile menu · clipboard · Bitcask simulator
   ========================================================================== */

document.addEventListener("DOMContentLoaded", () => {
    initTabs();
    initMobileMenu();
    initCopyButtons();
    initSimulator();
});

/* --------------------------------------------------------------------------
   Tabs
   -------------------------------------------------------------------------- */
function initTabs() {
    const tabs = Array.from(document.querySelectorAll(".arch-tab"));

    const activate = (tab) => {
        tabs.forEach((t) => {
            const selected = t === tab;
            t.setAttribute("aria-selected", String(selected));
            t.tabIndex = selected ? 0 : -1;
            const pane = document.getElementById(t.getAttribute("aria-controls"));
            if (pane) pane.hidden = !selected;
        });
    };

    tabs.forEach((tab, i) => {
        tab.addEventListener("click", () => activate(tab));
        tab.addEventListener("keydown", (e) => {
            let next = null;
            if (e.key === "ArrowRight") next = tabs[(i + 1) % tabs.length];
            else if (e.key === "ArrowLeft") next = tabs[(i - 1 + tabs.length) % tabs.length];
            else if (e.key === "Home") next = tabs[0];
            else if (e.key === "End") next = tabs[tabs.length - 1];
            if (next) {
                e.preventDefault();
                activate(next);
                next.focus();
            }
        });
    });
}

/* --------------------------------------------------------------------------
   Mobile menu
   -------------------------------------------------------------------------- */
function initMobileMenu() {
    const toggle = document.getElementById("menu-toggle");
    const menu = document.getElementById("mobile-menu");
    if (!toggle || !menu) return;

    const setOpen = (open) => {
        toggle.setAttribute("aria-expanded", String(open));
        menu.hidden = !open;
    };

    toggle.addEventListener("click", () => {
        setOpen(toggle.getAttribute("aria-expanded") !== "true");
    });

    menu.querySelectorAll("a").forEach((link) => {
        link.addEventListener("click", () => setOpen(false));
    });

    document.addEventListener("keydown", (e) => {
        if (e.key === "Escape" && toggle.getAttribute("aria-expanded") === "true") {
            setOpen(false);
            toggle.focus();
        }
    });

    const desktop = window.matchMedia("(min-width: 861px)");
    const handle = (e) => { if (e.matches) setOpen(false); };
    if (desktop.addEventListener) desktop.addEventListener("change", handle);
    else if (desktop.addListener) desktop.addListener(handle);
}

/* --------------------------------------------------------------------------
   Clipboard
   -------------------------------------------------------------------------- */
function initCopyButtons() {
    document.querySelectorAll(".code-box").forEach((box) => {
        const btn = box.querySelector(".copy-btn");
        const code = box.querySelector("code");
        if (!btn || !code) return;

        btn.addEventListener("click", async () => {
            const text = code.innerText
                .split("\n")
                .filter((line) => !line.trim().startsWith("#"))
                .join("\n")
                .trim();

            try {
                await navigator.clipboard.writeText(text);
            } catch (err) {
                const range = document.createRange();
                range.selectNodeContents(code);
                const sel = window.getSelection();
                sel.removeAllRanges();
                sel.addRange(range);
            }

            const original = btn.textContent;
            btn.textContent = "Copied";
            btn.classList.add("is-copied");
            setTimeout(() => {
                btn.textContent = original;
                btn.classList.remove("is-copied");
            }, 1600);
        });
    });
}

/* --------------------------------------------------------------------------
   Simulator state
   -------------------------------------------------------------------------- */
let simIndex = new Map(); // key -> { fileID, valuePos, valueSize, timestamp, expiry }
let simSegments = {
    1: { id: 1, entries: [], size: 0, active: true }
};
let activeSegmentID = 1;
let nextSegmentID = 2;
const MAX_SEGMENT_ENTRIES = 4; // kept low so rotation is easy to watch

function initSimulator() {
    const set = document.getElementById("btn-set");
    const get = document.getElementById("btn-get");
    const del = document.getElementById("btn-del");
    const merge = document.getElementById("btn-merge");
    const reset = document.getElementById("btn-reset");
    const form = document.getElementById("sim-form");

    if (set) set.addEventListener("click", simSet);
    if (get) get.addEventListener("click", simGet);
    if (del) del.addEventListener("click", simDelete);
    if (merge) merge.addEventListener("click", simMerge);
    if (reset) reset.addEventListener("click", simReset);

    if (form) {
        form.addEventListener("submit", (e) => {
            e.preventDefault();
            simSet();
        });
    }

    setInterval(checkExpirations, 1000);
}

/* --------------------------------------------------------------------------
   Terminal logging — monochrome. Weight, case and inversion carry the state.
   -------------------------------------------------------------------------- */
const LOG_TAGS = {
    dim: "",
    info: "INFO",
    success: "OK",
    warning: "WARN",
    error: "ERR"
};

function logToTerminal(message, type = "dim") {
    const consoleEl = document.getElementById("sim-console");
    if (!consoleEl) return;

    const stamp = new Date().toLocaleTimeString([], { hour12: false });
    const line = document.createElement("div");
    line.className = `log-line text-${type}`;

    const tag = LOG_TAGS[type];
    line.innerHTML =
        `<span class="log-stamp">[${stamp}]</span> ` +
        (tag ? `<span class="log-tag">${tag}</span>` : "") +
        escapeHtml(message);

    consoleEl.appendChild(line);
    consoleEl.scrollTop = consoleEl.scrollHeight;
}

function escapeHtml(value) {
    return String(value)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
}

function generateMockCRC() {
    return Math.floor(Math.random() * 0xFFFFFFFF).toString(16).toUpperCase().padStart(8, "0");
}

/* --------------------------------------------------------------------------
   Rendering
   -------------------------------------------------------------------------- */
function renderHashTable() {
    const tbody = document.getElementById("index-table-body");
    if (!tbody) return;
    tbody.innerHTML = "";

    if (simIndex.size === 0) {
        tbody.innerHTML = '<tr class="empty-row"><td colspan="5">Index is empty. Set a key to begin.</td></tr>';
        return;
    }

    const now = Math.floor(Date.now() / 1000);

    simIndex.forEach((entry, key) => {
        const row = document.createElement("tr");
        row.id = `index-row-${key}`;

        let expiryStr = "Never";
        let expiryClass = "idx-expiry-none";
        if (entry.expiry > 0) {
            const diff = entry.expiry - now;
            expiryStr = diff > 0 ? `${diff}s left` : "Expired";
            expiryClass = "idx-expiry-live";
        }

        row.innerHTML = `
            <td class="idx-key">"${escapeHtml(key)}"</td>
            <td>${entry.fileID}</td>
            <td>${entry.valuePos} B</td>
            <td>${entry.valueSize} B</td>
            <td class="${expiryClass}">${expiryStr}</td>
        `;
        tbody.appendChild(row);
    });
}

function renderSegments() {
    const container = document.getElementById("segments-container");
    if (!container) return;
    container.innerHTML = "";

    Object.keys(simSegments)
        .map(Number)
        .sort((a, b) => a - b)
        .forEach((id) => {
            const seg = simSegments[id];

            const segEl = document.createElement("div");
            segEl.className = `sim-segment ${seg.active ? "active-seg" : ""}`;
            segEl.id = `segment-card-${id}`;

            const header = document.createElement("div");
            header.className = "seg-header";
            header.innerHTML = `
                <span class="seg-name">segment_${id}.log</span>
                <span class="seg-status ${seg.active ? "active" : "inactive"}">${seg.active ? "Active" : "Sealed"}</span>
            `;
            segEl.appendChild(header);

            const entriesEl = document.createElement("div");
            entriesEl.className = "seg-entries";
            entriesEl.id = `seg-entries-${id}`;

            if (seg.entries.length === 0) {
                entriesEl.innerHTML = '<div class="no-entries">No data written yet.</div>';
            } else {
                seg.entries.forEach((entry) => {
                    const recBox = document.createElement("div");
                    recBox.className = `record-box ${entry.isTombstone ? "tombstone-record" : ""}`;
                    recBox.id = `record-${id}-${entry.offset}`;

                    const valueChip = entry.isTombstone
                        ? ""
                        : `<span class="bin-part bin-val" title="Value data">"${escapeHtml(entry.value)}"</span>`;

                    const rawValue = entry.isTombstone ? "" : ` "${escapeHtml(entry.value)}"`;

                    recBox.innerHTML = `
                        <div class="record-offset">Offset ${entry.offset} B &middot; ${entry.size} B</div>
                        <div class="record-binary">
                            <span class="bin-part bin-crc" title="CRC32 checksum">CRC:${entry.crc}</span>
                            <span class="bin-part bin-ts" title="Unix timestamp">TS:${entry.timestamp}</span>
                            <span class="bin-part bin-ttl" title="Expiry timestamp">EXP:${entry.expiry}</span>
                            <span class="bin-part bin-ks" title="Key size">${entry.key.length}B</span>
                            <span class="bin-part bin-vs" title="Value size">${entry.valueSize}B</span>
                            <span class="bin-part bin-key" title="Key data">"${escapeHtml(entry.key)}"</span>
                            ${valueChip}
                        </div>
                        <div class="record-text">
                            Raw bytes:
                            <span class="raw-bytes">[0x${entry.crc}] [${entry.timestamp}] [${entry.expiry}] [${entry.key.length}] [${entry.valueSize}] "${escapeHtml(entry.key)}"${rawValue}</span>
                        </div>
                    `;
                    entriesEl.appendChild(recBox);
                });
            }

            segEl.appendChild(entriesEl);
            container.appendChild(segEl);
        });

    container.scrollTop = container.scrollHeight;
}

/* --------------------------------------------------------------------------
   Monochrome flashes — inversion for a write, a hairline tint for a read
   -------------------------------------------------------------------------- */
function flashElement(el, kind, duration = 1000) {
    if (!el) return;
    const cls = `flash-${kind}`;
    el.classList.add(cls);
    setTimeout(() => el.classList.remove(cls), duration);
}

/* --------------------------------------------------------------------------
   Expiry
   -------------------------------------------------------------------------- */
function checkExpirations() {
    const now = Math.floor(Date.now() / 1000);
    const expired = [];

    simIndex.forEach((entry, key) => {
        if (entry.expiry > 0 && entry.expiry <= now) expired.push(key);
    });

    if (expired.length > 0) {
        expired.forEach((key) => {
            simIndex.delete(key);
            logToTerminal(`Key '${key}' expired. Removed lazily from the in-memory index.`, "warning");
        });
        renderHashTable();
    } else if (simIndex.size > 0) {
        renderHashTable(); // refresh the countdowns
    }
}

/* --------------------------------------------------------------------------
   Segment rotation
   -------------------------------------------------------------------------- */
function rotateActiveSegment() {
    const oldID = activeSegmentID;
    simSegments[oldID].active = false;

    const newID = nextSegmentID++;
    simSegments[newID] = { id: newID, entries: [], size: 0, active: true };
    activeSegmentID = newID;

    logToTerminal(`segment_${oldID}.log reached capacity. Rotated to segment_${newID}.log`, "info");
}

function ensureCapacity() {
    if (simSegments[activeSegmentID].entries.length >= MAX_SEGMENT_ENTRIES) {
        rotateActiveSegment();
    }
    return simSegments[activeSegmentID];
}

/* --------------------------------------------------------------------------
   Commands
   -------------------------------------------------------------------------- */
function simSet() {
    const keyEl = document.getElementById("sim-key");
    const valEl = document.getElementById("sim-value");
    const ttlEl = document.getElementById("sim-ttl");

    const key = keyEl.value.trim();
    const val = valEl.value.trim();

    if (!key || !val) {
        logToTerminal("Key and value are both required for SET.", "error");
        return;
    }

    const now = Math.floor(Date.now() / 1000);
    let expiry = 0;
    if (ttlEl.value) expiry = now + parseInt(ttlEl.value, 10);

    const keySize = key.length;
    const valSize = val.length;
    const recordSize = 20 + keySize + valSize; // 20-byte header

    const activeSeg = ensureCapacity();
    const offset = activeSeg.size;

    activeSeg.entries.push({
        offset,
        crc: generateMockCRC(),
        timestamp: now,
        expiry,
        key,
        value: val,
        valueSize: valSize,
        size: recordSize,
        isTombstone: false
    });
    activeSeg.size += recordSize;

    simIndex.set(key, {
        fileID: activeSegmentID,
        valuePos: offset,
        valueSize: valSize,
        timestamp: now,
        expiry
    });

    logToTerminal(
        `SET '${key}' = '${val}' at offset ${offset} in segment_${activeSegmentID}.log` +
        (expiry ? ` (ttl ${ttlEl.value}s)` : ""),
        "success"
    );

    renderHashTable();
    renderSegments();

    keyEl.value = "";
    valEl.value = "";
    ttlEl.value = "";

    setTimeout(() => {
        flashElement(document.getElementById(`index-row-${key}`), "read", 800);
        flashElement(document.getElementById(`record-${activeSegmentID}-${offset}`), "write", 1000);
    }, 100);
}

function simGet() {
    const keyEl = document.getElementById("sim-key");
    const key = keyEl.value.trim();

    if (!key) {
        logToTerminal("A key is required for GET.", "error");
        return;
    }

    logToTerminal(`GET '${key}' — consulting the KeyDir index…`, "dim");

    const indexEntry = simIndex.get(key);
    if (!indexEntry) {
        logToTerminal(`GET '${key}' failed: key not present in the index. No disk read performed.`, "error");
        return;
    }

    const now = Math.floor(Date.now() / 1000);
    if (indexEntry.expiry > 0 && indexEntry.expiry <= now) {
        simIndex.delete(key);
        renderHashTable();
        logToTerminal(`GET '${key}' failed: the key expired.`, "error");
        return;
    }

    flashElement(document.getElementById(`index-row-${key}`), "read", 1100);

    const segment = simSegments[indexEntry.fileID];
    const record = segment && segment.entries.find((e) => e.offset === indexEntry.valuePos);

    if (record) {
        logToTerminal(`Verifying CRC32 [${record.crc}]… valid`, "info");
        logToTerminal(
            `GET '${key}' → '${record.value}' — one seek into segment_${indexEntry.fileID}.log at offset ${indexEntry.valuePos}`,
            "success"
        );
        setTimeout(() => {
            flashElement(
                document.getElementById(`record-${indexEntry.fileID}-${indexEntry.valuePos}`),
                "read",
                1100
            );
        }, 100);
        return;
    }

    logToTerminal("Corruption: the index points at a record that is not on disk.", "error");
}

function simDelete() {
    const keyEl = document.getElementById("sim-key");
    const key = keyEl.value.trim();

    if (!key) {
        logToTerminal("A key is required for DELETE.", "error");
        return;
    }

    if (!simIndex.has(key)) {
        logToTerminal(`DELETE '${key}' failed: key not found.`, "error");
        return;
    }

    const now = Math.floor(Date.now() / 1000);
    const recordSize = 20 + key.length; // header only, no value

    const activeSeg = ensureCapacity();
    const offset = activeSeg.size;

    activeSeg.entries.push({
        offset,
        crc: generateMockCRC(),
        timestamp: now,
        expiry: 0,
        key,
        value: "",
        valueSize: 0,
        size: recordSize,
        isTombstone: true
    });
    activeSeg.size += recordSize;

    simIndex.delete(key);

    logToTerminal(
        `DELETE '${key}': tombstone appended to segment_${activeSegmentID}.log at offset ${offset}`,
        "warning"
    );

    renderHashTable();
    renderSegments();

    setTimeout(() => {
        flashElement(document.getElementById(`record-${activeSegmentID}-${offset}`), "delete", 1000);
    }, 100);

    keyEl.value = "";
}

function simMerge() {
    logToTerminal("Compaction triggered. Scanning sealed segments…", "info");

    const inactiveIDs = Object.keys(simSegments)
        .map(Number)
        .filter((id) => id !== activeSegmentID);

    if (inactiveIDs.length === 0) {
        logToTerminal("Compaction cancelled: no sealed segments to merge.", "warning");
        return;
    }

    logToTerminal(`Segments queued for compaction: [${inactiveIDs.join(", ")}]`, "dim");

    const keysToCopy = [];
    simIndex.forEach((entry, key) => {
        if (inactiveIDs.includes(entry.fileID)) keysToCopy.push({ key, entry });
    });

    logToTerminal(
        `${keysToCopy.length} live record(s) salvaged. Tombstones and expired values discarded.`,
        "info"
    );

    const mergeSegID = nextSegmentID++;
    const mergeSeg = { id: mergeSegID, entries: [], size: 0, active: false };

    keysToCopy.forEach(({ key, entry }) => {
        const origRecord = simSegments[entry.fileID].entries.find((e) => e.offset === entry.valuePos);
        if (!origRecord) return;

        const offset = mergeSeg.size;
        mergeSeg.entries.push({ ...origRecord, offset });
        mergeSeg.size += origRecord.size;

        simIndex.set(key, { ...entry, fileID: mergeSegID, valuePos: offset });
        logToTerminal(`  → '${key}' copied to segment_${mergeSegID}.log at offset ${offset}`, "dim");
    });

    inactiveIDs.forEach((id) => {
        delete simSegments[id];
        logToTerminal(`  → segment_${id}.log deleted`, "warning");
    });

    simSegments[mergeSegID] = mergeSeg;

    logToTerminal(`Compaction complete. segment_${activeSegmentID}.log remains active.`, "success");

    renderHashTable();
    renderSegments();
}

function simReset() {
    simIndex.clear();
    simSegments = { 1: { id: 1, entries: [], size: 0, active: true } };
    activeSegmentID = 1;
    nextSegmentID = 2;

    const consoleEl = document.getElementById("sim-console");
    if (consoleEl) consoleEl.innerHTML = "";

    renderHashTable();
    renderSegments();
    logToTerminal("Database wiped and re-initialised.", "info");
}
