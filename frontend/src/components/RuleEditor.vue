<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import type { FieldSchema, Rule } from "../lib/schema";
import { validateRule } from "../lib/schema";
import Dialog from "./Dialog.vue";

const props = defineProps<{ rule: Rule; schema: FieldSchema[]; busy?: boolean }>();
const emit = defineEmits<{ save: [Rule]; toggle: [boolean]; reset: [] }>();

const draft = reactive<Rule>({ ...props.rule });
watch(
  () => [props.rule.id, props.rule.version],
  () => Object.assign(draft, props.rule),
);

const errors = ref<Record<string, string>>({});
const confirm = ref<"save" | "disable" | "reset" | "">("");
const summary = ref("");

const fields = computed(() => [
  { key: "service", label: "服务 *", type: "text" },
  { key: "resource", label: "资源 *", type: "text" },
  { key: "method", label: "方法", type: "text" },
  { key: "qps", label: "QPS 上限", type: "number" },
  { key: "mode", label: "模式", type: "mode" },
  { key: "error_rate", label: "熔断错误率", type: "number", step: "0.01" },
  { key: "min_requests", label: "最小样本", type: "number" },
  { key: "open_timeout_ms", label: "OPEN 时长 ms", type: "number" },
  { key: "half_open_probes", label: "半开探测数", type: "number" },
]);

function onSave() {
  const err = validateRule(draft, props.schema);
  errors.value = err;
  if (Object.keys(err).length) {
    summary.value = "请先修正标红字段";
    return;
  }
  summary.value = "";
  confirm.value = "save";
}

function commitSave() {
  confirm.value = "";
  emit("save", { ...draft });
}

function onToggle() {
  if (draft.enabled) confirm.value = "disable";
  else emit("toggle", true);
}

function commitDisable() {
  confirm.value = "";
  emit("toggle", false);
}
</script>

<template>
  <section class="rounded-2xl border border-line bg-panel/90 p-4 md:p-5" data-testid="rule-editor">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-medium text-white">动态规则下发</h2>
        <p class="font-mono text-[11px] text-fog/70">v{{ rule.version }} · {{ rule.updated_at || "尚未保存" }}</p>
      </div>
      <label class="flex items-center gap-2 text-xs">
        <span>启用保护</span>
        <input type="checkbox" :checked="draft.enabled" @change="onToggle" />
      </label>
    </div>
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <label v-for="f in fields" :key="f.key" class="block text-xs">
        <span class="mb-1 block text-fog">{{ f.label }}</span>
        <select v-if="f.type === 'mode'" v-model="draft.mode" class="w-full rounded-lg border border-line bg-ink px-3 py-2 text-sm text-white">
          <option value="fixed">fixed</option>
          <option value="adaptive">adaptive</option>
        </select>
        <input
          v-else
          class="w-full rounded-lg border bg-ink px-3 py-2 text-sm text-white"
          :class="errors[f.key] ? 'border-rose' : 'border-line'"
          :type="f.type"
          :step="f.step"
          v-model="draft[f.key as keyof Rule]"
        />
        <span v-if="errors[f.key]" class="mt-1 block text-rose">{{ errors[f.key] }}</span>
      </label>
    </div>
    <p v-if="summary" class="mt-3 text-sm text-rose" data-testid="form-summary">{{ summary }}</p>
    <div class="mt-4 flex flex-wrap gap-2">
      <button class="rounded-lg bg-cyan px-4 py-2 text-sm text-ink disabled:opacity-50" :disabled="busy" data-testid="save-rule" @click="onSave">
        保存并下发
      </button>
      <button class="rounded-lg border border-rose/50 px-4 py-2 text-sm text-rose" aria-label="重置熔断" data-testid="reset-circuit" @click="confirm = 'reset'">
        重置熔断
      </button>
    </div>
    <Dialog v-if="confirm === 'save'" title="确认下发规则？" @cancel="confirm = ''" @confirm="commitSave">
      将把 {{ draft.resource }} 的 QPS 更新为 {{ draft.qps }}，并广播到在线节点。
    </Dialog>
    <Dialog v-if="confirm === 'disable'" title="停用保护？" danger @cancel="confirm = ''" @confirm="commitDisable">
      停用后该资源不再限流/熔断，仅保留统计。
    </Dialog>
    <Dialog v-if="confirm === 'reset'" title="重置熔断状态？" danger @cancel="confirm = ''" @confirm="confirm = ''; emit('reset')">
      节点将把该资源熔断器强制拉回 CLOSED。
    </Dialog>
  </section>
</template>
