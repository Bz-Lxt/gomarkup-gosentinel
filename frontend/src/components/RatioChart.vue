<script setup lang="ts">
type Pt = { at: string; block_ratio: number; error_ratio: number; state?: string };
defineProps<{ points: Pt[] }>();
</script>

<template>
  <div class="h-40 w-full" data-testid="ratio-chart">
    <svg v-if="points.length" viewBox="0 0 100 40" class="h-full w-full" preserveAspectRatio="none">
      <template v-for="(p, i) in points" :key="i">
        <rect
          v-if="p.state === 'OPEN' || p.state === 'HALF_OPEN'"
          :x="(i / Math.max(points.length - 1, 1)) * 100"
          y="0"
          :width="100 / Math.max(points.length, 1)"
          height="40"
          fill="rgba(255,92,122,0.12)"
        />
      </template>
      <polyline
        fill="none"
        stroke="#f5b942"
        stroke-width="0.7"
        :points="points.map((p, i) => `${(i / Math.max(points.length - 1, 1)) * 100},${40 - p.block_ratio * 38}`).join(' ')"
      />
      <polyline
        fill="none"
        stroke="#ff5c7a"
        stroke-width="0.7"
        :points="points.map((p, i) => `${(i / Math.max(points.length - 1, 1)) * 100},${40 - p.error_ratio * 38}`).join(' ')"
      />
    </svg>
    <p v-else class="flex h-full items-center justify-center font-mono text-xs text-fog/60">等待时间序列</p>
  </div>
</template>
