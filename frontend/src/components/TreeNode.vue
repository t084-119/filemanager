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
      <span
        class="label"
        :class="{ truncate: node.type === 'file' }"
        :title="node.name"
      >
        {{ displayName }}
      </span>
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
const expanded = ref(false);

const fileIcon = computed(() => {
  if (!props.node.name) return '📄';
  const lower = props.node.name.toLowerCase();
  if (lower.endsWith('.md')) return '📝';
  if (/(\.png|\.jpg|\.jpeg|\.gif|\.webp|\.svg)$/i.test(lower)) return '🖼️';
  return '📄';
});

const displayName = computed(() => {
  if (!props.node.name) return '';
  if (props.node.type === 'dir') return props.node.name;
  const name = props.node.name;
  if (name.length <= 26) return name;
  const parts = name.split('.');
  if (parts.length < 2) {
    return `${name.slice(0, 12)}…${name.slice(-8)}`;
  }
  const ext = parts.pop();
  const base = parts.join('.');
  const head = base.slice(0, 10);
  const tail = base.slice(-6);
  return `${head}…${tail}.${ext}`;
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
  flex: 1;
  min-width: 0;
  font-size: 0.92rem;
  word-break: break-all;
}

.label.truncate {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.children {
  margin-left: 18px;
  border-left: 1px dashed rgba(148, 163, 184, 0.5);
  padding-left: 10px;
}
</style>
