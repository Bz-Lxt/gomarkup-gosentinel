<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";

const props = defineProps<{
  pass: number;
  block: number;
  fallback: number;
  connected: boolean;
}>();

type Blip = { a: number; r: number; kind: "pass" | "block" | "circuit"; born: number };
const wrap = ref<HTMLDivElement | null>(null);
const canvas = ref<HTMLCanvasElement | null>(null);
const blips: Blip[] = [];
let raf = 0;
let sweep = 0;

function spawn() {
  const total = props.pass + props.block + props.fallback || 1;
  const n = Math.min(18, 4 + Math.round((props.pass + props.block) / 30));
  for (let i = 0; i < n; i++) {
    const roll = Math.random() * total;
    let kind: Blip["kind"] = "pass";
    if (roll > props.pass + props.block) kind = "circuit";
    else if (roll > props.pass) kind = "block";
    blips.push({ a: Math.random() * Math.PI * 2, r: 0.25 + Math.random() * 0.7, kind, born: performance.now() });
  }
  while (blips.length > 80) blips.shift();
}

function draw() {
  const c = canvas.value;
  const box = wrap.value;
  if (!c || !box) return;
  const dpr = window.devicePixelRatio || 1;
  const w = box.clientWidth;
  const h = box.clientHeight;
  c.width = Math.floor(w * dpr);
  c.height = Math.floor(h * dpr);
  c.style.width = `${w}px`;
  c.style.height = `${h}px`;
  const ctx = c.getContext("2d");
  if (!ctx) return;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, w, h);
  const cx = w / 2;
  const cy = h / 2;
  const R = Math.min(w, h) * 0.42;
  ctx.strokeStyle = "rgba(62,224,200,0.18)";
  ctx.lineWidth = 1;
  for (let i = 1; i <= 4; i++) {
    ctx.beginPath();
    ctx.arc(cx, cy, (R * i) / 4, 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.beginPath();
  ctx.moveTo(cx - R, cy);
  ctx.lineTo(cx + R, cy);
  ctx.moveTo(cx, cy - R);
  ctx.lineTo(cx, cy + R);
  ctx.stroke();

  sweep += 0.018;
  const grad = ctx.createLinearGradient(cx, cy, cx + Math.cos(sweep) * R, cy + Math.sin(sweep) * R);
  grad.addColorStop(0, "rgba(62,224,200,0)");
  grad.addColorStop(1, "rgba(62,224,200,0.55)");
  ctx.fillStyle = grad;
  ctx.beginPath();
  ctx.moveTo(cx, cy);
  ctx.arc(cx, cy, R, sweep - 0.55, sweep);
  ctx.closePath();
  ctx.fill();

  const now = performance.now();
  for (const b of blips) {
    const age = (now - b.born) / 2200;
    if (age > 1) continue;
    const x = cx + Math.cos(b.a) * b.r * R;
    const y = cy + Math.sin(b.a) * b.r * R;
    ctx.globalAlpha = 1 - age;
    ctx.fillStyle = b.kind === "pass" ? "#5ee6a0" : b.kind === "block" ? "#f5b942" : "#ff5c7a";
    ctx.beginPath();
    ctx.arc(x, y, 3.2, 0, Math.PI * 2);
    ctx.fill();
    ctx.globalAlpha = 1;
  }
  if (!props.connected) {
    ctx.fillStyle = "rgba(7,11,20,0.35)";
    ctx.fillRect(0, 0, w, h);
  }
  raf = requestAnimationFrame(draw);
}

let timer = 0;
onMounted(() => {
  spawn();
  timer = window.setInterval(spawn, 700);
  const ro = new ResizeObserver(() => {});
  if (wrap.value) ro.observe(wrap.value);
  raf = requestAnimationFrame(draw);
  onUnmounted(() => {
    cancelAnimationFrame(raf);
    clearInterval(timer);
    ro.disconnect();
  });
});
watch(() => [props.pass, props.block, props.fallback], () => spawn());
</script>

<template>
  <div ref="wrap" class="relative h-full min-h-[280px] w-full" data-testid="radar">
    <canvas ref="canvas" class="h-full w-full"></canvas>
    <div class="pointer-events-none absolute inset-x-0 bottom-3 text-center font-mono text-[10px] uppercase tracking-[0.35em] text-cyan/70">
      sweep · live intercept
    </div>
  </div>
</template>
