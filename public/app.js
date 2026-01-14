async function loadData() {
    const res = await fetch("/api/data");
    const data = await res.json();
    document.getElementById("out").textContent = JSON.stringify(data, null, 2);
}

async function addKV() {
    const k = document.getElementById("key").value.trim();
    const v = document.getElementById("value").value.trim();
    if (!k) return;

    const body = {};
    body[k] = v;

    const res = await fetch("/api/data", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body)
    });

    document.getElementById("out").textContent = await res.text();
    loadData();
}

async function delKV() {
    const k = document.getElementById("delKey").value.trim();
    if (!k) return;

    const res = await fetch("/api/data/" + encodeURIComponent(k), { method: "DELETE" });
    document.getElementById("out").textContent = await res.text();
    loadData();
}

async function loadStats() {
    const res = await fetch("/api/stats");
    const data = await res.json();
    document.getElementById("stats").textContent = JSON.stringify(data, null, 2);
}