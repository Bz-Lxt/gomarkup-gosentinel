<script setup lang="ts">
defineProps<{ text: string }>();
const emit = defineEmits<{ close: [] }>();
</script>

<template>
  <div class="fixed right-4 top-4 z-40 flex max-w-sm items-start gap-3 rounded-xl border border-rose/40 bg-panel px-4 py-3 text-sm text-white shadow-glow" role="alert">
    <p class="flex-1">{{ text }}</p>
    <button class="text-fog hover:text-white" aria-label="关闭提示" @click="emit('close')">×</button>
  </div>
</template>
