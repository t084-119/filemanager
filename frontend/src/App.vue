<template>
  <div class="page" :class="codeThemeClass">
    <main class="layout" :class="{ 'sidebar-hidden': !sidebarVisible }">
      <div v-if="sidebarVisible" class="sidebar-wrapper" :style="wrapperStyle">
        <section ref="sidebarRef" class="sidebar card">
          <div class="sidebar-header">
            <button class="refresh" @click="fetchTree" :disabled="loading">
              {{ loading ? '刷新中...' : '刷新目录' }}
            </button>
          </div>
          <div class="path-info">
            <span>当前目录:</span>
            <strong>{{ currentDir || '/' }}</strong>
          </div>
          <div class="upload-box">
            <input ref="fileInput" type="file" @change="onFileChange" />
            <button @click="upload" :disabled="!uploadFile || uploading">
              {{ uploading ? '上传中...' : '上传到当前目录' }}
            </button>
          </div>
          <div class="manage-actions">
            <button class="secondary" @click="createFolder">新建文件夹</button>
            <div class="file-create">
              <select v-model="createFileType">
                <option value="md">Markdown</option>
                <option value="txt">TXT</option>
                <option value="json">JSON</option>
              </select>
              <button class="secondary" @click="createFile">新建文件</button>
            </div>
            <button
              class="danger"
              @click="deleteSelected"
              :disabled="!selectedNode || selectedNode.path === ''"
            >
              删除选中
            </button>
          </div>
          <div class="tree-container" v-if="tree">
            <TreeNode
              :node="tree"
              :selected-path="selectedPath"
              @select="selectNode"
            />
          </div>
          <div class="empty" v-else>暂无目录数据</div>
        </section>
        <button class="sidebar-toggle collapse" @click="toggleSidebar">&lt;</button>
      </div>
      <button v-else class="sidebar-toggle expand" @click="toggleSidebar">&gt;</button>

      <section ref="contentRef" class="content card" :style="contentStyle">
        <div
          v-if="fileType === 'markdown'"
          class="content-resizer"
          @mousedown="startPreviewResize"
          title="拖动调整预览宽度"
        ></div>
        <div class="status" v-if="error">{{ error }}</div>
        <div v-if="selectedFile">
          <div class="preview-header">
            <div class="preview-title">
              <div class="section-title">文件预览</div>
              <h2 class="file-name" v-if="selectedFile">{{ selectedFile.name }}</h2>
            </div>
            <div class="preview-actions">
              <div class="theme-toggle" v-if="fileType === 'markdown'">
                <span>代码主题</span>
                <button @click="toggleTheme">
                  {{ codeTheme === 'light' ? '夜间' : '白天' }}
                </button>
              </div>
              <div class="actions" v-if="isEditable">
                <button @click="toggleEdit">
                  {{ isEditing ? '取消编辑' : '编辑内容' }}
                </button>
                <button v-if="isEditing" class="primary" @click="saveFile" :disabled="saving">
                  {{ saving ? '保存中...' : '保存' }}
                </button>
              </div>
            </div>
          </div>
          <div class="preview-body">
            <div v-if="fileType === 'image'" class="image-preview">
              <img :src="imageUrl" :alt="selectedFile.name" />
            </div>

            <div v-else-if="fileType === 'markdown'" class="markdown-area">
              <textarea
                v-if="isEditing"
                v-model="editContent"
                class="editor"
              ></textarea>
              <div v-else ref="previewRef" class="markdown" v-html="renderedMarkdown"></div>
            </div>

            <div v-else-if="fileType === 'pdf'" class="pdf-preview">
              <iframe :src="imageUrl" title="PDF预览"></iframe>
            </div>

            <div v-else class="text-preview">
              <textarea v-if="isEditing" v-model="editContent" class="editor"></textarea>
              <pre v-else>{{ fileContent }}</pre>
            </div>
          </div>
        </div>
        <div v-else>
          <div class="preview-header">
            <div class="preview-title">
              <div class="section-title">文件预览</div>
            </div>
          </div>
          <div class="preview-body">
            <div class="empty">请选择左侧目录树中的文件进行预览。</div>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import axios from 'axios';
import hljs from 'highlight.js';
import mermaid from 'mermaid';
import { marked } from 'marked';
import TreeNode from './components/TreeNode.vue';

const tree = ref(null);
const selectedFile = ref(null);
const selectedPath = ref('');
const selectedNode = ref(null);
const currentDir = ref('');
const fileContent = ref('');
const fileType = ref('');
const imageUrl = ref('');
const loading = ref(false);
const error = ref('');
const uploadFile = ref(null);
const uploading = ref(false);
const isEditing = ref(false);
const editContent = ref('');
const saving = ref(false);
const previewRef = ref(null);
const fileInput = ref(null);
const codeTheme = ref('light');
const createFileType = ref('md');
const sidebarVisible = ref(true);
const sidebarWidth = ref(320);
const sidebarHeight = ref(360);
const sidebarRef = ref(null);
const contentRef = ref(null);
const resizeObserver = ref(null);
const contentWidth = ref(null);
const isResizingPreview = ref(false);
const resizeState = ref({ startX: 0, startWidth: 0 });

const codeThemeClass = computed(() =>
  codeTheme.value === 'light' ? 'code-theme-light' : 'code-theme-dark'
);
const wrapperStyle = computed(() => ({
  '--sidebar-width': `${sidebarWidth.value}px`
}));
const isEditable = computed(() =>
  ['markdown', 'text', 'json'].includes(fileType.value)
);
const contentStyle = computed(() => {
  if (!sidebarVisible.value) {
    return { maxWidth: '100%', width: '100%', justifySelf: 'stretch' };
  }
  if (contentWidth.value) {
    return { width: `${contentWidth.value}px`, maxWidth: 'none' };
  }
  return {};
});

const renderer = new marked.Renderer();
const escapeHtml = (value) =>
  value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/\"/g, '&quot;')
    .replace(/'/g, '&#39;');

renderer.code = (code, info) => {
  const language = (info || '').trim().toLowerCase();
  if (language === 'mermaid') {
    return `<div class="mermaid">${code}</div>`;
  }
  const hasLanguage = language && hljs.getLanguage(language);
  const highlighted = hasLanguage
    ? hljs.highlight(code, { language }).value
    : hljs.highlightAuto(code).value;
  const langLabel = language || 'text';
  return `<pre><code class="hljs language-${langLabel}" data-lang="${langLabel}">${highlighted}</code></pre>`;
};

const currentFileDir = ref('');
const resolveAssetPath = (href) => {
  if (!href) return '';
  if (/^(https?:)?\/\//i.test(href)) {
    return href;
  }
  const baseParts = currentFileDir.value ? currentFileDir.value.split('/') : [];
  const relParts = href.split('/');
  const stack = [...baseParts];
  for (const part of relParts) {
    if (!part || part === '.') continue;
    if (part === '..') {
      stack.pop();
      continue;
    }
    stack.push(part);
  }
  const resolved = stack.join('/');
  return `/api/raw?path=${encodeURIComponent(resolved)}`;
};

renderer.image = (href, title, text) => {
  const safeTitle = title ? ` title="${escapeHtml(title)}"` : '';
  const safeAlt = text ? escapeHtml(text) : '';
  const resolved = resolveAssetPath(href);
  return `<img src="${resolved}" alt="${safeAlt}"${safeTitle} />`;
};
marked.setOptions({ renderer, breaks: true });

mermaid.initialize({ startOnLoad: false, theme: 'default' });

const renderedMarkdown = computed(() => {
  if (fileType.value !== 'markdown') return '';
  return marked.parse(fileContent.value || '');
});

const fetchTree = async () => {
  loading.value = true;
  error.value = '';
  try {
    const response = await axios.get('/api/tree');
    tree.value = response.data;
  } catch (err) {
    error.value = '获取目录失败，请确认后端已启动。';
  } finally {
    loading.value = false;
  }
};

const selectNode = async (node) => {
  selectedNode.value = node;
  selectedPath.value = node.path;
  if (node.type === 'dir') {
    currentDir.value = node.path || '';
    currentFileDir.value = node.path || '';
    selectedFile.value = null;
    fileType.value = '';
    return;
  }

  selectedFile.value = node;
  currentDir.value = node.path.split('/').slice(0, -1).join('/');
  currentFileDir.value = currentDir.value;
  error.value = '';
  isEditing.value = false;
  try {
    const response = await axios.get('/api/file', {
      params: { path: node.path }
    });
    fileContent.value = response.data.content;
    fileType.value = response.data.type;
    if (fileType.value === 'image' || fileType.value === 'pdf') {
      imageUrl.value = `/api/raw?path=${encodeURIComponent(node.path)}`;
    }
  } catch (err) {
    error.value = '无法加载文件内容。';
  }
};

const onFileChange = (event) => {
  uploadFile.value = event.target.files[0] || null;
};

const upload = async () => {
  if (!uploadFile.value) return;
  uploading.value = true;
  const form = new FormData();
  form.append('file', uploadFile.value);
  try {
    await axios.post('/api/upload', form, {
      params: { path: currentDir.value }
    });
    await fetchTree();
    if (fileInput.value) {
      fileInput.value.value = '';
    }
  } catch (err) {
    error.value = err?.response?.data?.error
      ? `上传失败：${err.response.data.error}`
      : '上传失败，请重试。';
  } finally {
    uploading.value = false;
    uploadFile.value = null;
  }
};

const createFolder = async () => {
  const name = window.prompt('请输入新建文件夹名称');
  if (!name) return;
  const parent = selectedNode.value?.type === 'dir' ? selectedNode.value.path : currentDir.value;
  try {
    await axios.post('/api/create', {
      parent,
      name,
      type: 'dir'
    });
    await fetchTree();
  } catch (err) {
    error.value = err?.response?.data?.error
      ? `创建失败：${err.response.data.error}`
      : '创建失败，请重试。';
  }
};

const createFile = async () => {
  const type = createFileType.value;
  const name = window.prompt('请输入文件名');
  if (!name) return;
  const parent = selectedNode.value?.type === 'dir' ? selectedNode.value.path : currentDir.value;
  const extension = type === 'md' ? 'md' : type;
  const finalName = name.includes('.') ? name : `${name}.${extension}`;
  let content = '';
  if (type === 'md') {
    content = '# 新建文档\n\n请在此编写内容。';
  } else if (type === 'json') {
    content = '{\n  \"name\": \"example\"\n}\n';
  }
  try {
    await axios.post('/api/create', {
      parent,
      name: finalName,
      type: 'file',
      content
    });
    await fetchTree();
  } catch (err) {
    error.value = err?.response?.data?.error
      ? `创建失败：${err.response.data.error}`
      : '创建失败，请重试。';
  }
};

const deleteSelected = async () => {
  if (!selectedNode.value) return;
  if (selectedNode.value.path === '') {
    error.value = '根目录不可删除。';
    return;
  }
  if (!window.confirm(`确定要删除 ${selectedNode.value.name} 吗？`)) return;
  try {
    await axios.delete('/api/file', { params: { path: selectedNode.value.path } });
    selectedNode.value = null;
    selectedFile.value = null;
    selectedPath.value = '';
    fileType.value = '';
    await fetchTree();
  } catch (err) {
    error.value = err?.response?.data?.error
      ? `删除失败：${err.response.data.error}`
      : '删除失败，请重试。';
  }
};

const toggleEdit = () => {
  isEditing.value = !isEditing.value;
  if (isEditing.value) {
    editContent.value = fileContent.value;
  }
};

const saveFile = async () => {
  if (!selectedFile.value) return;
  saving.value = true;
  try {
    await axios.put(`/api/file?path=${encodeURIComponent(selectedFile.value.path)}`, editContent.value, {
      headers: { 'Content-Type': 'text/plain' }
    });
    fileContent.value = editContent.value;
    const ext = selectedFile.value.name?.split('.').pop()?.toLowerCase();
    if (ext === 'md' || ext === 'markdown') {
      fileType.value = 'markdown';
    } else if (ext === 'json') {
      fileType.value = 'json';
    } else if (ext === 'txt') {
      fileType.value = 'text';
    }
    isEditing.value = false;
  } catch (err) {
    error.value = '保存失败，请重试。';
  } finally {
    saving.value = false;
  }
};

watch(renderedMarkdown, async () => {
  await nextTick();
  if (previewRef.value) {
    const nodes = previewRef.value.querySelectorAll('.mermaid');
    if (nodes.length) {
      mermaid.run({ nodes });
    }
    const images = previewRef.value.querySelectorAll('img');
    images.forEach((img) => {
      const src = img.getAttribute('src') || '';
      if (!src || src.startsWith('data:')) return;
      if (/^(https?:)?\/\//i.test(src)) return;
      if (src.startsWith('/api/raw')) return;
      const resolved = resolveAssetPath(src);
      img.setAttribute('src', resolved);
    });
  }
});

onMounted(() => {
  fetchTree();
  nextTick(() => {
    updateSidebarMetrics();
    updateContentMetrics();
    if (sidebarRef.value && contentRef.value) {
      resizeObserver.value = new ResizeObserver(() => {
        updateSidebarMetrics();
        updateContentMetrics();
      });
      resizeObserver.value.observe(sidebarRef.value);
      resizeObserver.value.observe(contentRef.value);
    }
  });
});

onBeforeUnmount(() => {
  if (resizeObserver.value) {
    resizeObserver.value.disconnect();
  }
  window.removeEventListener('mousemove', handlePreviewResize);
  window.removeEventListener('mouseup', stopPreviewResize);
});

const toggleTheme = () => {
  codeTheme.value = codeTheme.value === 'light' ? 'dark' : 'light';
};

const updateSidebarMetrics = () => {
  const element = sidebarRef.value;
  if (!element) return;
  const rect = element.getBoundingClientRect();
  sidebarWidth.value = rect.width;
  sidebarHeight.value = rect.height;
};

const updateContentMetrics = () => {
  const element = contentRef.value;
  if (!element) return;
  if (!sidebarVisible.value) {
    element.style.maxWidth = '100%';
    contentWidth.value = null;
    return;
  }
  const availableWidth = window.innerWidth - sidebarWidth.value - 96;
  const maxWidth = Math.max(360, availableWidth);
  element.style.maxWidth = `${maxWidth}px`;
  if (contentWidth.value && contentWidth.value > maxWidth) {
    contentWidth.value = maxWidth;
  }
};

const toggleSidebar = () => {
  sidebarVisible.value = !sidebarVisible.value;
  if (sidebarVisible.value) {
    nextTick(() => {
      updateSidebarMetrics();
      updateContentMetrics();
    });
  } else {
    nextTick(() => updateContentMetrics());
  }
};

const startPreviewResize = (event) => {
  if (!contentRef.value) return;
  isResizingPreview.value = true;
  const rect = contentRef.value.getBoundingClientRect();
  resizeState.value = { startX: event.clientX, startWidth: rect.width };
  window.addEventListener('mousemove', handlePreviewResize);
  window.addEventListener('mouseup', stopPreviewResize);
};

const handlePreviewResize = (event) => {
  if (!isResizingPreview.value) return;
  const delta = resizeState.value.startX - event.clientX;
  const availableWidth = window.innerWidth - (sidebarVisible.value ? sidebarWidth.value : 0) - 96;
  const maxWidth = Math.max(360, availableWidth);
  const nextWidth = Math.min(Math.max(360, resizeState.value.startWidth + delta), maxWidth);
  contentWidth.value = nextWidth;
};

const stopPreviewResize = () => {
  isResizingPreview.value = false;
  window.removeEventListener('mousemove', handlePreviewResize);
  window.removeEventListener('mouseup', stopPreviewResize);
};
</script>

<style scoped>
.page {
  min-height: 100vh;
  height: 100vh;
  background: radial-gradient(circle at top, #eef2ff 0%, #f8fafc 45%, #f1f5f9 100%);
  padding: 32px;
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  color: #0f172a;
  overflow: hidden;
}

.refresh {
  background: #4f46e5;
  border: none;
  color: white;
  padding: 8px 14px;
  border-radius: 10px;
  cursor: pointer;
  font-weight: 600;
  transition: transform 0.2s ease;
}

.refresh:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.layout {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 24px;
  align-items: stretch;
  position: relative;
  padding-left: 0;
  height: calc(100vh - 64px);
}

.layout.sidebar-hidden {
  grid-template-columns: 1fr;
}

.layout.sidebar-hidden .content {
  max-width: 100%;
  justify-self: stretch;
}

.layout:not(.sidebar-hidden) {
  padding-left: var(--sidebar-width, 360px);
}

.card {
  background: white;
  border-radius: 18px;
  padding: 20px;
  box-shadow: 0 20px 45px rgba(15, 23, 42, 0.08);
  border: 1px solid rgba(148, 163, 184, 0.15);
}

.sidebar-wrapper {
  position: relative;
  width: 0;
  height: 0;
}

.sidebar {
  position: fixed;
  top: 24px;
  left: 32px;
  height: calc(100vh - 40px);
  overflow: auto;
  resize: both;
  min-width: 260px;
  max-width: 520px;
  width: 320px;
  min-height: 360px;
  display: flex;
  flex-direction: column;
}

.sidebar-toggle {
  border: none;
  background: #1e293b;
  color: white;
  width: 28px;
  height: 48px;
  border-radius: 999px;
  cursor: pointer;
  font-size: 1rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.2);
}

.sidebar-toggle.collapse {
  position: fixed;
  top: 140px;
  left: calc(var(--sidebar-width, 320px) + 20px);
  z-index: 5;
}

.sidebar-toggle.expand {
  position: fixed;
  top: 160px;
  left: 12px;
  z-index: 5;
}

.section-title {
  font-weight: 700;
  margin-bottom: 0;
  font-size: 1.1rem;
  color: #1e293b;
}

.sidebar-header {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.path-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 16px;
  color: #64748b;
}

.upload-box {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.upload-box button {
  border: none;
  background: #22c55e;
  color: white;
  padding: 10px;
  border-radius: 10px;
  cursor: pointer;
  font-weight: 600;
}

.upload-box button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.manage-actions {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  margin-bottom: 16px;
}

.file-create {
  display: flex;
  gap: 6px;
  align-items: center;
}

.file-create select {
  flex: 1;
  border: 1px solid #cbd5f5;
  border-radius: 10px;
  padding: 6px 8px;
  color: #4338ca;
  background: #eef2ff;
  font-weight: 600;
}

.manage-actions .secondary {
  border: 1px solid #cbd5f5;
  background: #eef2ff;
  color: #4338ca;
  padding: 8px 10px;
  border-radius: 10px;
  cursor: pointer;
  font-weight: 600;
}

.manage-actions .danger {
  border: none;
  background: #ef4444;
  color: white;
  padding: 8px 10px;
  border-radius: 10px;
  cursor: pointer;
  font-weight: 600;
}

.manage-actions button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.tree-container {
  flex: 1;
  min-height: 0;
  max-height: none;
  overflow: auto;
}

.content {
  position: relative;
  resize: none;
  overflow: hidden;
  min-width: 360px;
  max-width: calc(100vw - var(--sidebar-width, 320px) - 96px);
  max-height: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-self: end;
  min-height: 0;
}

.content.card {
  border: 1px solid rgba(148, 163, 184, 0.35);
}

.content-resizer {
  position: absolute;
  top: 16px;
  left: -8px;
  width: 16px;
  height: calc(100% - 32px);
  cursor: ew-resize;
  z-index: 2;
}

.content-resizer::before {
  content: '';
  position: absolute;
  left: 7px;
  top: 0;
  width: 2px;
  height: 100%;
  background: rgba(148, 163, 184, 0.65);
  border-radius: 999px;
}

.preview-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  background: white;
  padding-bottom: 12px;
  margin-bottom: 20px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.3);
  gap: 16px;
  flex-shrink: 0;
}

.preview-body {
  flex: 1 1 auto;
  overflow-y: auto;
  padding-right: 8px;
  min-height: 0;
}

.preview-title {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.file-name {
  margin: 0;
  font-size: 1.6rem;
  color: #0f172a;
}

.preview-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.theme-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #475569;
}

.theme-toggle button {
  border: none;
  background: #0f172a;
  color: white;
  padding: 6px 12px;
  border-radius: 999px;
  cursor: pointer;
}

.subtitle {
  margin: 4px 0 0;
  color: #94a3b8;
  font-size: 0.85rem;
}

.actions button {
  margin-left: 8px;
  border: 1px solid #cbd5f5;
  background: white;
  padding: 8px 12px;
  border-radius: 8px;
  cursor: pointer;
}

.actions .primary {
  background: #4f46e5;
  color: white;
  border: none;
}

.image-preview img {
  width: 100%;
  border-radius: 12px;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.15);
}

.pdf-preview iframe {
  width: 100%;
  height: 70vh;
  border: none;
  border-radius: 12px;
  background: #f8fafc;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.12);
}

.markdown-area {
  line-height: 1.7;
}

.markdown h1,
.markdown h2,
.markdown h3 {
  margin-top: 1.2em;
}

.markdown h4,
.markdown h5,
.markdown h6 {
  margin-top: 1em;
}

.markdown ul,
.markdown ol {
  margin: 0.6em 0 0.6em 1.6em;
  padding-left: 1.4em;
}

.markdown li {
  margin: 0.35em 0;
}

.markdown ul ul,
.markdown ol ol,
.markdown ul ol,
.markdown ol ul {
  margin-top: 0.3em;
}

.markdown h1 + ul,
.markdown h2 + ul,
.markdown h3 + ul,
.markdown h4 + ul,
.markdown h5 + ul,
.markdown h6 + ul,
.markdown h1 + ol,
.markdown h2 + ol,
.markdown h3 + ol,
.markdown h4 + ol,
.markdown h5 + ol,
.markdown h6 + ol {
  margin-top: 0.4em;
  margin-left: 1.8em;
}

.markdown code:not(pre code) {
  background: rgba(99, 102, 241, 0.12);
  color: #4338ca;
  padding: 2px 6px;
  border-radius: 6px;
  font-size: 0.92em;
}

.markdown pre {
  background: transparent;
  padding: 12px;
  border-radius: 10px;
  overflow: auto;
}

.editor {
  width: 100%;
  min-height: 320px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 12px;
  font-family: 'Fira Code', monospace;
}

.text-preview pre {
  background: #f1f5f9;
  padding: 12px;
  border-radius: 10px;
}

.status {
  background: #fee2e2;
  color: #991b1b;
  padding: 10px;
  border-radius: 10px;
  margin-bottom: 12px;
}

.empty {
  color: #94a3b8;
}

@media (max-width: 1024px) {
  .layout {
    grid-template-columns: 1fr;
  }

  .sidebar {
    position: static;
    max-height: none;
  }

  .content {
    max-width: 100%;
  }
}
</style>
