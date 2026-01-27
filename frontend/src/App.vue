<template>
  <div>
    <header class="header">本地文件管理</header>
    <main class="container">
      <section class="panel">
        <h3>目录与文件</h3>
        <div class="actions-row">
          <input v-model="newDirName" class="input" placeholder="新建目录名" />
          <button class="button secondary" :disabled="!newDirName" @click="createDir">新建目录</button>
        </div>
        <div class="actions-column">
          <input v-model="newMdName" class="input" placeholder="新建 Markdown 文件名 (例如 note.md)" />
          <textarea v-model="newMdContent" class="textarea" rows="5" placeholder="写入 Markdown 内容"></textarea>
          <button class="button secondary" :disabled="!newMdName" @click="createMarkdown">新建 Markdown</button>
        </div>
        <div class="upload">
          <input type="file" @change="onFileChange" accept=".md,.markdown,.png,.jpg,.jpeg,.gif,.webp" />
          <button class="button" :disabled="!selectedFile || uploading" @click="uploadFile">
            {{ uploading ? '上传中...' : '上传文件' }}
          </button>
        <span class="notice">文件将上传到当前目录，支持 Markdown 与图片格式</span>
        </div>
        <div class="tree" v-if="tree.length">
          <TreeItem
            v-for="node in tree"
            :key="node.path"
            :node="node"
            :level="0"
            :active-path="activeFile?.path"
            :current-dir="currentDir"
            @select-file="selectFile"
            @select-dir="selectDir"
          />
        </div>
        <div class="empty" v-else>暂无文件</div>
      </section>
      <section class="panel">
        <h3>文件预览</h3>
        <div class="preview-actions" v-if="activeFile">
          <button class="button danger" @click="removeFile(activeFile)">删除当前文件</button>
        </div>
        <div class="viewer" v-if="activeFile">
          <template v-if="activeFile.type === 'markdown'">
            <div class="markdown" v-html="markdownHtml"></div>
          </template>
          <template v-else>
            <img :src="fileUrl(activeFile.path)" :alt="activeFile.name" />
          </template>
        </div>
        <div class="empty" v-else>请选择一个文件进行预览</div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, defineComponent, nextTick, onMounted, ref } from 'vue'
import { marked } from 'marked'
import mermaid from 'mermaid'

const tree = ref([])
const activeFile = ref(null)
const selectedFile = ref(null)
const uploading = ref(false)
const markdownHtml = ref('')
const currentDir = ref('')

const newDirName = ref('')
const newMdName = ref('')
const newMdContent = ref('')

const fetchTree = async () => {
  const response = await fetch('/api/tree')
  if (!response.ok) {
    throw new Error('无法获取文件列表')
  }
  tree.value = await response.json()
  if (activeFile.value) {
    const file = findFile(tree.value, activeFile.value.path)
    if (!file) {
      activeFile.value = null
      markdownHtml.value = ''
    }
  }
}

const findFile = (nodes, path) => {
  for (const node of nodes) {
    if (node.type === 'dir' && node.children) {
      const found = findFile(node.children, path)
      if (found) return found
    } else if (node.path === path) {
      return node
    }
  }
  return null
}

const selectFile = async (file) => {
  activeFile.value = file
  currentDir.value = parentDir(file.path)
  if (file.type === 'markdown') {
    const response = await fetch(fileUrl(file.path))
    const text = await response.text()
    markdownHtml.value = marked.parse(text)
    await nextTick()
    mermaid.init(undefined, document.querySelectorAll('.mermaid'))
  } else {
    markdownHtml.value = ''
  }
}

const selectDir = (dirPath) => {
  currentDir.value = dirPath
}

const onFileChange = (event) => {
  const file = event.target.files?.[0]
  selectedFile.value = file || null
}

const uploadFile = async () => {
  if (!selectedFile.value) return
  uploading.value = true
  try {
    const formData = new FormData()
    formData.append('file', selectedFile.value)
    formData.append('path', currentDir.value)
    const response = await fetch('/api/upload', {
      method: 'POST',
      body: formData
    })
    if (!response.ok) {
      throw new Error('上传失败')
    }
    selectedFile.value = null
    await fetchTree()
  } catch (error) {
    alert(error.message)
  } finally {
    uploading.value = false
  }
}

const createDir = async () => {
  const target = joinPath(currentDir.value, newDirName.value.trim())
  const response = await fetch('/api/dir', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path: target })
  })
  if (!response.ok) {
    alert('创建目录失败')
    return
  }
  newDirName.value = ''
  await fetchTree()
}

const createMarkdown = async () => {
  const target = joinPath(currentDir.value, newMdName.value.trim())
  const response = await fetch('/api/md', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path: target, content: newMdContent.value })
  })
  if (!response.ok) {
    alert('创建 Markdown 失败')
    return
  }
  newMdName.value = ''
  newMdContent.value = ''
  await fetchTree()
}

const removeFile = async (file) => {
  if (!confirm(`确认删除 ${file.name} 吗？`)) {
    return
  }
  const response = await fetch(`/api/files/${encodeURIComponent(file.path)}`, {
    method: 'DELETE'
  })
  if (!response.ok) {
    alert('删除失败')
    return
  }
  if (activeFile.value?.path === file.path) {
    activeFile.value = null
    markdownHtml.value = ''
  }
  await fetchTree()
}

const joinPath = (dir, name) => {
  if (!dir) return name
  return `${dir.replace(/\/$/, '')}/${name}`
}

const parentDir = (path) => {
  const trimmed = path.replace(/\/[^/]+$/, '')
  return trimmed
}

const fileUrl = (path) => `/files/${encodeURIComponent(path)}`

const TreeItem = defineComponent({
  props: {
    node: { type: Object, required: true },
    level: { type: Number, required: true },
    activePath: { type: String, default: '' },
    currentDir: { type: String, default: '' }
  },
  emits: ['select-file', 'select-dir'],
  setup(props, { emit }) {
    const expanded = ref(true)
    const isDir = computed(() => props.node.type === 'dir')
    const padding = computed(() => ({ paddingLeft: `${props.level * 16}px` }))

    const toggle = () => {
      if (isDir.value) {
        expanded.value = !expanded.value
      }
    }

    const select = () => {
      if (isDir.value) {
        emit('select-dir', props.node.path)
      } else {
        emit('select-file', props.node)
      }
    }

    return { expanded, isDir, padding, toggle, select }
  },
  template: `
    <div>
      <div
        class="tree-item"
        :class="{ active: !isDir && activePath === node.path, selected: isDir && currentDir === node.path }"
        :style="padding"
      >
        <button class="tree-toggle" v-if="isDir" @click.stop="toggle">
          {{ expanded ? '▾' : '▸' }}
        </button>
        <span class="tree-icon" @click="select">{{ isDir ? '📁' : '📄' }}</span>
        <span class="tree-name" @click="select">{{ node.name }}</span>
      </div>
      <div v-if="isDir && expanded">
        <TreeItem
          v-for="child in node.children"
          :key="child.path"
          :node="child"
          :level="level + 1"
          :active-path="activePath"
          :current-dir="currentDir"
          @select-file="$emit('select-file', $event)"
          @select-dir="$emit('select-dir', $event)"
        />
      </div>
    </div>
  `
})

const formatMarkdown = () => {
  const renderer = new marked.Renderer()
  renderer.code = (code, lang) => {
    if (lang === 'mermaid') {
      return `<div class="mermaid">${code}</div>`
    }
    return `<pre><code>${code}</code></pre>`
  }
  marked.setOptions({ renderer })
}

onMounted(() => {
  mermaid.initialize({ startOnLoad: false })
  formatMarkdown()
  fetchTree().catch((error) => alert(error.message))
})
</script>
