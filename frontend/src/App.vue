<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import Radar from "./components/Radar.vue";
import RatioChart from "./components/RatioChart.vue";
import RuleEditor from "./components/RuleEditor.vue";
import Toast from "./components/Toast.vue";
import { api } from "./lib/api";
import { formatBeijing } from "./lib/time";
import type { FieldSchema, Rule } from "./lib/schema";

const conn = ref<"ok" | "reconnecting" | "polling" | "error" | "loading">("loading");
const toast = ref("");
let toastTimer = 0;
const overview = ref<any>({ summary: {}, nodes: [], convergence: {}, version: 0 });
const metrics = ref<any[]>([]);
const rules = ref<Rule[]>([]);
const schema = ref<FieldSchema[]>([]);
const selectedId = ref("");
const service = ref("");
const resource = ref("");
const instance = ref("");
const range = ref("60");
const lastTick = ref("");
const seenAt = new Set<string>();

const selected = computed(() => rules.value.find((r) => r.id === selectedId.value) || rules.value[0]);
const sum = computed(() => overview.value.summary || {});
const pass = computed(() => Number(sum.value.pass || 0));
const block = computed(() => Number(sum.value.block || 0));
const fallback = computed(() => Number(sum.value.fallback || 0));
const total = computed(() => pass.value + block.value);
const passRatio = computed(() => (total.value ? pass.value / total.value : 0));
const blockRatio = computed(() => Number(sum.value.block_ratio || 0));
const errRatio = computed(() => Number(sum.value.error_ratio || 0));
const online = computed(() => (overview.value.nodes || []).filter((n: any) => n.connected).length);
const chartPoints = computed(() => {
  const pts: { at: string; block_ratio: number; error_ratio: number; state?: string }[] = [];
  for (const m of metrics.value) {
    for (const p of m.series || []) {
      if (seenAt.has(p.at + m.resource)) continue;
      seenAt.add(p.at + m.resource);
      pts.push(p);
    }
  }
  return pts.slice(-60);
});

function showError(e: unknown) {
  toast.value = e instanceof Error ? e.message : "请求失败";
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toast.value = ""), 5000);
}

async function refresh() {
  try {
    const q = new URLSearchParams();
    if (service.value) q.set("service", service.value);
    if (resource.value) q.set("resource", resource.value);
    if (instance.value) q.set("instance", instance.value);
    q.set("range", `${range.value}s`);
    const [ov, ms, rs] = await Promise.all([api.overview(), api.metrics(`?${q}`), api.rules()]);
    overview.value = ov;
    metrics.value = ms;
    rules.value = rs.rules || [];
    if (!selectedId.value && rules.value[0]) selectedId.value = rules.value[0].id;
    lastTick.value = formatBeijing();
    if (conn.value !== "ok") conn.value = conn.value === "reconnecting" ? "polling" : conn.value === "loading" ? "polling" : conn.value;
  } catch (e) {
    conn.value = "error";
    showError(e);
  }
}

let ws: WebSocket | null = null;
let poll = 0;
let retry = 0;

function connectWS() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws/dashboard`);
  ws.onopen = () => {
    conn.value = "ok";
    retry = 0;
  };
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data);
      if (msg.type === "dashboard_tick" && msg.payload) {
        overview.value = { ...overview.value, ...msg.payload };
        if (msg.payload.metrics) metrics.value = msg.payload.metrics;
        lastTick.value = formatBeijing();
      }
    } catch {
      /* ignore malformed */
    }
  };
  ws.onclose = () => {
    conn.value = "reconnecting";
    retry += 1;
    window.setTimeout(connectWS, Math.min(8000, 400 * 2 ** retry));
  };
}

async function saveRule(rule: Rule) {
  try {
    const res: any = await api.saveRule(rule);
    const conv = res.convergence || res.rule && {};
    await refresh();
    toast.value = `已下发 v${res.rule?.version || res.version || ""} · ACK ${conv.ack ?? "?"} / ${conv.target_nodes ?? "?"}`;
    window.setTimeout(() => (toast.value = ""), 5000);
  } catch (e) {
    showError(e);
  }
}

onMounted(async () => {
  try {
    schema.value = await api.schema();
  } catch (e) {
    showError(e);
  }
  await refresh();
  connectWS();
  poll = window.setInterval(refresh, 2000);
});
onUnmounted(() => {
  window.clearInterval(poll);
  ws?.close();
});
</script>

<template>
  <div class="min-h-screen w-full px-3 py-4 sm:px-5 lg:px-6">
    <Toast v-if="toast" :text="toast" @close="toast = ''" />
    <header class="mb-4 flex w-full flex-wrap items-center justify-between gap-3">
      <div>
        <p class="font-mono text-[11px] uppercase tracking-[0.35em] text-cyan">Mini Sentinel</p>
        <h1 class="text-2xl text-white sm:text-3xl">阻断雷达控制室</h1>
      </div>
      <div class="flex items-center gap-3 font-mono text-xs" data-testid="conn-status">
        <span
          class="rounded-full px-3 py-1"
          :class="{
            'bg-mint/15 text-mint': conn === 'ok',
            'bg-amber/15 text-amber': conn === 'reconnecting' || conn === 'polling',
            'bg-rose/15 text-rose': conn === 'error',
            'bg-line text-fog': conn === 'loading',
          }"
        >
          {{ conn === "ok" ? "WS LIVE" : conn === "reconnecting" ? "RECONNECTING" : conn === "polling" ? "HTTP POLL" : conn === "loading" ? "LOADING" : "ERROR" }}
        </span>
        <span>{{ lastTick }}</span>
      </div>
    </header>

    <div v-if="conn === 'loading'" class="rounded-2xl border border-line bg-panel p-10 text-center">正在同步控制面…</div>
    <div v-else-if="conn === 'error' && !rules.length" class="rounded-2xl border border-rose/40 bg-panel p-10 text-center text-rose">
      控制面不可达，请确认 docker compose 已启动。
    </div>

    <div v-else class="grid w-full grid-cols-12 gap-4">
      <section class="col-span-12 rounded-2xl border border-line bg-panel/80 p-3 shadow-glow lg:col-span-5">
        <Radar :pass="pass" :block="block" :fallback="fallback" :connected="conn === 'ok' || conn === 'polling'" />
      </section>
      <section class="col-span-12 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:col-span-7">
        <article class="rounded-2xl border border-line bg-panel p-4">
          <p class="font-mono text-[11px] text-fog">Pass</p>
          <p class="text-2xl text-mint">{{ (passRatio * 100).toFixed(1) }}%</p>
          <p class="text-xs">{{ pass }}</p>
        </article>
        <article class="rounded-2xl border border-line bg-panel p-4">
          <p class="font-mono text-[11px] text-fog">Block</p>
          <p class="text-2xl text-amber">{{ (blockRatio * 100).toFixed(1) }}%</p>
          <p class="text-xs">{{ block }}</p>
        </article>
        <article class="rounded-2xl border border-line bg-panel p-4">
          <p class="font-mono text-[11px] text-fog">Circuit / Fallback</p>
          <p class="text-2xl text-rose">{{ fallback }}</p>
          <p class="text-xs">error {{ (errRatio * 100).toFixed(1) }}%</p>
        </article>
        <article class="rounded-2xl border border-line bg-panel p-4">
          <p class="font-mono text-[11px] text-fog">瞬时 QPS</p>
          <p class="text-2xl text-white">{{ (pass + block).toFixed(0) }}</p>
        </article>
        <article class="rounded-2xl border border-line bg-panel p-4">
          <p class="font-mono text-[11px] text-fog">在线节点</p>
          <p class="text-2xl text-cyan">{{ online }}</p>
        </article>
        <article class="rounded-2xl border border-line bg-panel p-4" data-testid="convergence">
          <p class="font-mono text-[11px] text-fog">规则收敛</p>
          <p class="text-2xl text-white">v{{ overview.version || 0 }}</p>
          <p class="text-xs">ACK {{ overview.convergence?.ack ?? 0 }}/{{ overview.convergence?.target_nodes ?? 0 }} · 未收敛 {{ overview.convergence?.not_converged ?? 0 }}</p>
        </article>
      </section>

      <section class="col-span-12 rounded-2xl border border-line bg-panel p-4 lg:col-span-7">
        <div class="mb-3 flex flex-wrap items-end gap-3">
          <label class="text-xs">服务
            <input v-model="service" class="mt-1 block rounded-lg border border-line bg-ink px-3 py-2" data-testid="filter-service" />
          </label>
          <label class="text-xs">资源
            <input v-model="resource" class="mt-1 block rounded-lg border border-line bg-ink px-3 py-2" />
          </label>
          <label class="text-xs">实例
            <input v-model="instance" class="mt-1 block rounded-lg border border-line bg-ink px-3 py-2" />
          </label>
          <label class="text-xs">窗口
            <select v-model="range" class="mt-1 block rounded-lg border border-line bg-ink px-3 py-2">
              <option value="30">30s</option>
              <option value="60">60s</option>
              <option value="180">180s</option>
            </select>
          </label>
          <button class="rounded-lg border border-line px-3 py-2 text-xs" data-testid="apply-filter" @click="refresh">筛选</button>
        </div>
        <RatioChart :points="chartPoints" />
        <div class="mt-2 flex gap-4 font-mono text-[11px]">
          <span class="text-amber">■ block ratio</span>
          <span class="text-rose">■ error ratio</span>
          <span class="text-rose/50">■ 熔断区间</span>
        </div>
        <div v-if="!metrics.length" class="mt-4 text-center text-sm text-fog/60">暂无遥测。对示例服务加压后将出现轨迹。</div>
      </section>

      <div class="col-span-12 lg:col-span-5">
        <label class="mb-2 block text-xs">选择规则
          <select v-model="selectedId" class="mt-1 w-full rounded-lg border border-line bg-ink px-3 py-2 text-sm">
            <option v-for="r in rules" :key="r.id" :value="r.id">{{ r.service }} {{ r.resource }}</option>
          </select>
        </label>
        <RuleEditor
          v-if="selected"
          :rule="selected"
          :schema="schema"
          @save="saveRule"
          @toggle="(en) => selected && api.patchRule(selected.id, { enabled: en }).then(refresh).catch(showError)"
          @reset="selected && api.resetRule(selected.id).then(refresh).catch(showError)"
        />
        <p v-else class="rounded-2xl border border-line bg-panel p-8 text-center text-sm">没有规则，控制面将在首次启动时写入默认保护项。</p>
      </div>
    </div>
  </div>
</template>
