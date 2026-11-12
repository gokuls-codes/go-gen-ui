const sendBtn = document.getElementById("send-btn");

document.addEventListener("htmx:beforeRequest", (e) => {
  if (e.target.id === "chat-form") sendBtn.disabled = true;
});

document.addEventListener("htmx:afterRequest", (e) => {
  if (e.detail.pathInfo?.requestPath.startsWith("/chat/reply")) {
    sendBtn.disabled = false;
  }
});

document.addEventListener("htmx:responseError", () => {
  sendBtn.disabled = false;
});

function cssVar(name, fallback) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
}

function initCharts(root = document) {
  if (typeof Chart === "undefined") return;

  const palette = [
    cssVar("--chart-1", "#38bdf8"),
    cssVar("--chart-2", "#22d3ee"),
    cssVar("--chart-3", "#0ea5e9"),
    cssVar("--chart-4", "#0284c7"),
    cssVar("--chart-5", "#0369a1"),
  ];
  const cardBg = cssVar("--card", "#1e293b");
  const borderColor = cssVar("--border", "rgba(255,255,255,0.1)");
  const mutedText = cssVar("--muted-foreground", "#94a3b8");

  root.querySelectorAll("canvas[data-chart]").forEach((canvas) => {
    if (canvas.dataset.chartInit) return;
    canvas.dataset.chartInit = "true";

    const cfg = JSON.parse(canvas.dataset.chart);
    const isCircular = cfg.type === "doughnut" || cfg.type === "pie";
    const colors = cfg.values.map((_, i) => palette[i % palette.length]);

    new Chart(canvas, {
      type: cfg.type || "bar",
      data: {
        labels: cfg.labels,
        datasets: [
          {
            label: cfg.title || "Value",
            data: cfg.values,
            backgroundColor: cfg.type === "line" ? palette[0] : colors,
            borderColor: cfg.type === "line" ? palette[0] : isCircular ? cardBg : colors,
            borderWidth: isCircular ? 2 : cfg.type === "line" ? 2 : 0,
            borderRadius: cfg.type === "bar" ? 6 : 0,
            tension: 0.35,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: {
            display: isCircular,
            position: "bottom",
            labels: { color: mutedText, boxWidth: 12, padding: 14 },
          },
          tooltip: {
            callbacks: {
              label: (ctx) => {
                const display = cfg.displays?.[ctx.dataIndex];
                const val = display || ctx.formattedValue;
                return `${ctx.label}: ${val}`;
              },
            },
          },
        },
        scales: isCircular
          ? {}
          : {
              x: {
                ticks: { color: mutedText },
                grid: { display: false },
              },
              y: {
                beginAtZero: true,
                ticks: { color: mutedText },
                grid: { color: borderColor },
              },
            },
      },
    });
  });
}

document.addEventListener("DOMContentLoaded", () => initCharts());
document.addEventListener("htmx:afterSettle", () => initCharts());
