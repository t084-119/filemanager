<template>
  <div class="tree-node">
    <div
      class="node-row"
      :class="{ active: node.path === selectedPath }"
      @click="handleClick"
    >
      <span class="caret" v-if="node.type === 'dir'">
        {{ expanded ? '▾' : '▸' }}
      </span>
      <span class="caret" v-else>•</span>
      <span class="icon">{{ node.type === 'dir' ? '📁' : fileIcon }}</span>
      <span class="label">{{ node.name }}</span>
    </div>
    <div v-if="node.type === 'dir' && expanded" class="children">
      <TreeNode
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :selected-path="selectedPath"
        @select="emit('select', $event)"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';

const props = defineProps({
  node: {
    type: Object,
    required: true
  },
  selectedPath: {
    type: String,
    default: ''
  }
});

const emit = defineEmits(['select']);
const expanded = ref(true);

const fileIcon = computed(() => {
  if (!props.node.name) return '📄';
  const lower = props.node.name.toLowerCase();
  if (lower.endsWith('.md')) return '📝';
  if (/(\.png|\.jpg|\.jpeg|\.gif|\.webp|\.svg)$/i.test(lower)) return '🖼️';
  return '📄';
});

const handleClick = () => {
  if (props.node.type === 'dir') {
    expanded.value = !expanded.value;
  }
  emit('select', props.node);
};
</script>

<style scoped>
.tree-node {
  margin-left: 4px;
}

.node-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s ease;
}

.node-row:hover {
  background: rgba(79, 70, 229, 0.08);
}

.node-row.active {
  background: rgba(79, 70, 229, 0.16);
  color: #3730a3;
  font-weight: 600;
}

.caret {
  width: 16px;
  display: inline-flex;
  justify-content: center;
  color: #6b7280;
}

.icon {
  width: 22px;
  display: inline-flex;
}

.label {
  font-size: 0.92rem;
  word-break: break-all;
}

.children {
  margin-left: 18px;
  border-left: 1px dashed rgba(148, 163, 184, 0.5);
  padding-left: 10px;
}
</style>
