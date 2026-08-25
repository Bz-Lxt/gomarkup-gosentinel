<script setup lang="ts">
defineProps<{ title: string; danger?: boolean }>();
const emit = defineEmits<{ cancel: []; confirm: [] }>();
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
    <div class="w-full max-w-md rounded-2xl border border-line bg-panel p-5 shadow-glow">
      <h3 class="text-lg font-medium text-white">{{ title }}</h3>
      <div class="mt-3 text-sm text-fog">
        <slot />
      </div>
      <div class="mt-5 flex justify-end gap-3">
        <button class="rounded-lg border border-line px-4 py-2 text-sm hover:bg-line" @click="emit('cancel')">取消</button>
        <button
          class="rounded-lg px-4 py-2 text-sm text-ink"
          :class="danger ? 'bg-rose' : 'bg-cyan'"
          @click="emit('confirm')"
        >
          确认
        </button>
      </div>
    </div>
  </div>
</template>
