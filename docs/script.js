// JS Controller for LagerKV Store Documentation Page & Simulator

// 1. Architecture Tabs controller
document.addEventListener("DOMContentLoaded", () => {
    const tabs = document.querySelectorAll(".arch-tab");
    const panes = document.querySelectorAll(".tab-pane");

    tabs.forEach(tab => {
        tab.addEventListener("click", () => {
            tabs.forEach(t => t.classList.remove("active"));
            panes.forEach(p => p.classList.remove("active"));

            tab.classList.add("active");
            const targetId = tab.getAttribute("data-tab");
            document.getElementById(targetId).classList.add("active");
        });
    });

    // Start Expiry checker for the simulator
    setInterval(checkExpirations, 1000);
});

// 2. Bitcask Simulator State
let simIndex = new Map(); // Key -> { fileID, valuePos, valueSize, timestamp, expiry }
let simSegments = {
    1: {
        id: 1,
        entries: [], // list of { offset, crc, timestamp, expiry, key, value, size, isTombstone }
        size: 0,
        active: true
    }
};
let activeSegmentID = 1;
let nextSegmentID = 2;
const MAX_SEGMENT_ENTRIES = 4; // Low number for easy visualization

// Terminal Logging helper
function logToTerminal(message, type = "dim") {
    const consoleEl = document.getElementById("sim-console");
    const timestamp = new Date().toLocaleTimeString();
    const line = document.createElement("div");
    line.className = `log-line text-${type}`;
    line.innerHTML = `<span class="text-dim">[${timestamp}]</span> ${message}`;
    consoleEl.appendChild(line);
    consoleEl.scrollTop = consoleEl.scrollHeight;
}

// Generate a mock CRC32 hex string
function generateMockCRC() {
    return Math.floor(Math.random() * 0xFFFFFFFF).toString(16).toUpperCase().padStart(8, '0');
}

// Render the HashTable UI
function renderHashTable() {
    const tbody = document.getElementById("index-table-body");
    tbody.innerHTML = "";

    if (simIndex.size === 0) {
        tbody.innerHTML = `<tr class="empty-row"><td colspan="5">Index is empty. Set a key to begin.</td></tr>`;
        return;
    }

    const now = Math.floor(Date.now() / 1000);
    simIndex.forEach((entry, key) => {
        const row = document.createElement("tr");
        row.id = `index-row-${key}`;

        let expiryStr = "Never";
        if (entry.expiry > 0) {
            const diff = entry.expiry - now;
            expiryStr = diff > 0 ? `${diff}s remaining` : "Expired";
        }

        row.innerHTML = `
            <td style="font-weight: 600; color: #a5b4fc;">"${key}"</td>
            <td>${entry.fileID}</td>
            <td>${entry.valuePos} bytes</td>
            <td>${entry.valueSize} bytes</td>
            <td style="color: ${entry.expiry > 0 ? '#fbbf24' : '#64748b'}">${expiryStr}</td>
        `;
        tbody.appendChild(row);
    });
}

// Render Segment Logs UI
function renderSegments() {
    const container = document.getElementById("segments-container");
    container.innerHTML = "";

    Object.keys(simSegments).sort((a, b) => a - b).forEach(id => {
        const seg = simSegments[id];
        const segEl = document.createElement("div");
        segEl.className = `sim-segment ${seg.active ? 'active-seg' : ''}`;
        segEl.id = `segment-card-${id}`;

        let statusText = seg.active ? 'Active' : 'Inactive';
        let statusClass = seg.active ? 'active' : 'inactive';

        const header = document.createElement("div");
        header.className = "seg-header";
        header.innerHTML = `
            <span class="seg-name">segment_${id}.log</span>
            <span class="seg-status ${statusClass}">${statusText}</span>
        `;
        segEl.appendChild(header);

        const entriesEl = document.createElement("div");
        entriesEl.className = "seg-entries";
        entriesEl.id = `seg-entries-${id}`;

        if (seg.entries.length === 0) {
            entriesEl.innerHTML = `<div class="no-entries">No data written yet.</div>`;
        } else {
            seg.entries.forEach(entry => {
                const recBox = document.createElement("div");
                recBox.className = `record-box ${entry.isTombstone ? 'tombstone-record' : ''}`;
                recBox.id = `record-${id}-${entry.offset}`;

                let expiryLabel = entry.expiry > 0 ? `Expiry: ${entry.expiry}` : "Expiry: None";
                recBox.innerHTML = `
                    <div class="record-offset">Offset: ${entry.offset} bytes | Size: ${entry.size} bytes</div>
                    <div class="record-binary">
                        <span class="bin-part bin-crc" title="CRC32 Checksum">CRC:${entry.crc}</span>
                        <span class="bin-part bin-ts" title="Unix Timestamp">TS:${entry.timestamp}</span>
                        <span class="bin-part bin-ttl" title="Expiration Timestamp">EXP:${entry.expiry}</span>
                        <span class="bin-part bin-ks" title="Key Size in Bytes">KS:${entry.key.length}B</span>
                        <span class="bin-part bin-vs" title="Value Size in Bytes">VS:${entry.valueSize}B</span>
                        <span class="bin-part bin-key" title="Key data">"${entry.key}"</span>
                        ${!entry.isTombstone ? `<span class="bin-part bin-val" title="Value data">"${entry.value}"</span>` : ''}
                    </div>
                    <div class="record-text">
                        <strong>Raw Bytes:</strong> 
                        <span style="font-family: var(--font-mono); font-size: 0.72rem; color: var(--text-secondary);">
                            [0x${entry.crc}] [${entry.timestamp}] [${entry.expiry}] [${entry.key.length}] [${entry.valueSize}] "${entry.key}" ${!entry.isTombstone ? `"${entry.value}"` : ''}
                        </span>
                    </div>
                `;
                entriesEl.appendChild(recBox);
            });
        }

        segEl.appendChild(entriesEl);
        container.appendChild(segEl);
    });

    // Auto-scroll segments visualizer to the bottom
    container.scrollTop = container.scrollHeight;
}

// Check expiration of keys
function checkExpirations() {
    const now = Math.floor(Date.now() / 1000);
    let expiredKeys = [];

    simIndex.forEach((entry, key) => {
        if (entry.expiry > 0 && entry.expiry <= now) {
            expiredKeys.push(key);
        }
    });

    if (expiredKeys.length > 0) {
        expiredKeys.forEach(key => {
            simIndex.delete(key);
            logToTerminal(`Key '${key}' expired. Lazily removed from In-Memory Index.`, "warning");
        });
        renderHashTable();
    } else if (simIndex.size > 0) {
        // Just re-render table to decrement countdowns
        renderHashTable();
    }
}

// Rotate segment
function rotateActiveSegment() {
    const oldID = activeSegmentID;
    simSegments[oldID].active = false;
    
    const newID = nextSegmentID++;
    simSegments[newID] = {
        id: newID,
        entries: [],
        size: 0,
        active: true
    };
    activeSegmentID = newID;

    logToTerminal(`Segment_${oldID}.log reached capacity. Rotated to active Segment_${newID}.log`, "info");
}

// Form logic actions: SET
function simSet() {
    const keyEl = document.getElementById("sim-key");
    const valEl = document.getElementById("sim-value");
    const ttlEl = document.getElementById("sim-ttl");

    const key = keyEl.value.trim();
    const val = valEl.value.trim();
    if (!key || !val) {
        logToTerminal("ERROR: Key and Value are required for SET.", "error");
        return;
    }

    const now = Math.floor(Date.now() / 1000);
    let expiry = 0;
    if (ttlEl.value) {
        expiry = now + parseInt(ttlEl.value);
    }

    // Binary format layout sizes
    const keySize = key.length;
    const valSize = val.length;
    const recordSize = 20 + keySize + valSize; // Header is 20 bytes

    const currentSeg = simSegments[activeSegmentID];
    
    // Rotate segment if active segment exceeds capacity
    if (currentSeg.entries.length >= MAX_SEGMENT_ENTRIES) {
        rotateActiveSegment();
    }

    const activeSeg = simSegments[activeSegmentID];
    const offset = activeSeg.size;
    const crc = generateMockCRC();

    const record = {
        offset: offset,
        crc: crc,
        timestamp: now,
        expiry: expiry,
        key: key,
        value: val,
        valueSize: valSize,
        size: recordSize,
        isTombstone: false
    };

    activeSeg.entries.push(record);
    activeSeg.size += recordSize;

    // Update index
    simIndex.set(key, {
        fileID: activeSegmentID,
        valuePos: offset,
        valueSize: valSize,
        timestamp: now,
        expiry: expiry
    });

    logToTerminal(`SET key='${key}' value='${val}' at offset=${offset} in segment_${activeSegmentID}.log (CRC Checked)`, "success");

    // UI Animations
    renderHashTable();
    renderSegments();

    // Reset fields
    keyEl.value = "";
    valEl.value = "";
    ttlEl.value = "";

    // Highlight visual additions
    setTimeout(() => {
        const row = document.getElementById(`index-row-${key}`);
        if (row) {
            row.classList.add("glowing-highlight");
            setTimeout(() => row.classList.remove("glowing-highlight"), 1000);
        }

        const box = document.getElementById(`record-${activeSegmentID}-${offset}`);
        if (box) {
            box.classList.add("glowing-highlight");
            setTimeout(() => box.classList.remove("glowing-highlight"), 1000);
        }
    }, 100);
}

// GET Key
function simGet() {
    const keyEl = document.getElementById("sim-key");
    const key = keyEl.value.trim();

    if (!key) {
        logToTerminal("ERROR: Key is required for GET.", "error");
        return;
    }

    logToTerminal(`GET key='${key}'... querying HashTable index.`, "dim");

    const indexEntry = simIndex.get(key);
    if (!indexEntry) {
        logToTerminal(`GET key='${key}' failed: Key not found in index.`, "error");
        return;
    }

    const now = Math.floor(Date.now() / 1000);
    if (indexEntry.expiry > 0 && indexEntry.expiry <= now) {
        simIndex.delete(key);
        renderHashTable();
        logToTerminal(`GET key='${key}' failed: Key has expired.`, "error");
        return;
    }

    // Highlight row in index table
    const row = document.getElementById(`index-row-${key}`);
    if (row) {
        row.classList.add("glowing-highlight-blue");
        setTimeout(() => row.classList.remove("glowing-highlight-blue"), 1200);
    }

    // Lookup record in segment
    const segment = simSegments[indexEntry.fileID];
    if (segment) {
        const record = segment.entries.find(e => e.offset === indexEntry.valuePos);
        if (record) {
            // Verify checksum simulation
            logToTerminal(`Verifying record CRC32 [${record.crc}]... OK`, "info");
            logToTerminal(`GET key='${key}' successful -> value='${record.value}' (Read from segment_${indexEntry.fileID}.log at offset=${indexEntry.valuePos})`, "success");

            // Highlight record box on disk
            setTimeout(() => {
                const box = document.getElementById(`record-${indexEntry.fileID}-${indexEntry.valuePos}`);
                if (box) {
                    box.classList.add("glowing-highlight-blue");
                    setTimeout(() => box.classList.remove("glowing-highlight-blue"), 1200);
                }
            }, 100);
            return;
        }
    }

    logToTerminal(`ERROR: Disk corruption. Reference index exists, but segment data is missing!`, "error");
}

// DELETE Key
function simDelete() {
    const keyEl = document.getElementById("sim-key");
    const key = keyEl.value.trim();

    if (!key) {
        logToTerminal("ERROR: Key is required for DELETE.", "error");
        return;
    }

    const indexEntry = simIndex.get(key);
    if (!indexEntry) {
        logToTerminal(`DELETE key='${key}' failed: Key not found.`, "error");
        return;
    }

    const now = Math.floor(Date.now() / 1000);
    const keySize = key.length;
    const recordSize = 20 + keySize; // Header is 20, value is empty

    const currentSeg = simSegments[activeSegmentID];
    if (currentSeg.entries.length >= MAX_SEGMENT_ENTRIES) {
        rotateActiveSegment();
    }

    const activeSeg = simSegments[activeSegmentID];
    const offset = activeSeg.size;
    const crc = generateMockCRC();

    const tombstoneRecord = {
        offset: offset,
        crc: crc,
        timestamp: now,
        expiry: 0,
        key: key,
        value: "",
        valueSize: 0,
        size: recordSize,
        isTombstone: true
    };

    activeSeg.entries.push(tombstoneRecord);
    activeSeg.size += recordSize;

    // Delete key from HashTable
    simIndex.delete(key);

    logToTerminal(`DELETE key='${key}': Appended Tombstone record to segment_${activeSegmentID}.log at offset=${offset}`, "warning");

    renderHashTable();
    renderSegments();

    // Highlight the tombstone box
    setTimeout(() => {
        const box = document.getElementById(`record-${activeSegmentID}-${offset}`);
        if (box) {
            box.classList.add("glowing-highlight-red");
            setTimeout(() => box.classList.remove("glowing-highlight-red"), 1000);
        }
    }, 100);

    keyEl.value = "";
}

// Trigger Compaction (Merge)
function simMerge() {
    logToTerminal(`Compaction triggered. Scanning inactive segments...`, "info");

    const inactiveIDs = Object.keys(simSegments).map(Number).filter(id => id !== activeSegmentID);
    if (inactiveIDs.length === 0) {
        logToTerminal(`Compaction canceled: No inactive segment logs to merge.`, "warning");
        return;
    }

    logToTerminal(`Compacting segments: [${inactiveIDs.join(", ")}]`, "dim");

    // Gather live keys remaining in the index that map to these inactive segments
    let keysToCopy = [];
    simIndex.forEach((entry, key) => {
        if (inactiveIDs.includes(entry.fileID)) {
            keysToCopy.push({ key, entry });
        }
    });

    logToTerminal(`Identified ${keysToCopy.length} live records to salvage. All tombstones and expired values will be discarded.`, "info");

    // Create a temporary merge segment
    const mergeSegID = nextSegmentID++;
    let mergeSeg = {
        id: mergeSegID,
        entries: [],
        size: 0,
        active: false
    };

    keysToCopy.forEach(item => {
        const origSegment = simSegments[item.entry.fileID];
        const origRecord = origSegment.entries.find(e => e.offset === item.entry.valuePos);

        if (origRecord) {
            const offset = mergeSeg.size;
            const newRecord = {
                ...origRecord,
                offset: offset
            };
            mergeSeg.entries.push(newRecord);
            mergeSeg.size += newRecord.size;

            // Update in-memory index pointing to the new merged file ID and new offset
            simIndex.set(item.key, {
                ...item.entry,
                fileID: mergeSegID,
                valuePos: offset
            });

            logToTerminal(`  -> Copying live key '${item.key}' to merged segment_${mergeSegID}.log at offset=${offset}`, "dim");
        }
    });

    // Delete old inactive segments
    inactiveIDs.forEach(id => {
        delete simSegments[id];
        logToTerminal(`  -> Deleted stale segment_${id}.log`, "warning");
    });

    // Save the merged segment
    simSegments[mergeSegID] = mergeSeg;

    logToTerminal(`Compaction complete! Space reclaimed. Active segment remains segment_${activeSegmentID}.log`, "success");

    renderHashTable();
    renderSegments();
}

// Reset Simulator
function simReset() {
    simIndex.clear();
    simSegments = {
        1: {
            id: 1,
            entries: [],
            size: 0,
            active: true
        }
    };
    activeSegmentID = 1;
    nextSegmentID = 2;

    const consoleEl = document.getElementById("sim-console");
    consoleEl.innerHTML = '<div class="log-line text-dim">Database initialized. Ready.</div>';
    
    renderHashTable();
    renderSegments();
    logToTerminal("Database state wiped and re-initialized.", "info");
}

// CSS Highlight injector helper
const styleSheet = document.createElement("style");
styleSheet.innerText = `
.glowing-highlight {
    background-color: rgba(16, 185, 129, 0.25) !important;
    box-shadow: 0 0 15px rgba(16, 185, 129, 0.4) !important;
    transition: all 0.2s ease;
}
.glowing-highlight-blue {
    background-color: rgba(99, 102, 241, 0.3) !important;
    box-shadow: 0 0 15px rgba(99, 102, 241, 0.5) !important;
    transition: all 0.2s ease;
}
.glowing-highlight-red {
    background-color: rgba(239, 68, 68, 0.3) !important;
    box-shadow: 0 0 15px rgba(239, 68, 68, 0.5) !important;
    transition: all 0.2s ease;
}
`;
document.head.appendChild(styleSheet);
