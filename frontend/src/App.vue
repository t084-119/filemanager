<template>
  <div class="page">
    <header class="hero">
      <div>
        <h1>本地文件管理中心</h1>
        <p>浏览目录树、渲染 Markdown + Mermaid、上传文件并实时预览。</p>
      </div>
      <button class="refresh" @click="fetchTree" :disabled="loading">
        {{ loading ? '刷新中...' : '刷新目录' }}
      </button>
    </header>

    <main class="layout">
      <section class="sidebar card">
        <div class="section-title">目录结构</div>
        <div class="path-info">
          <span>当前目录:</span>
          <strong>{{ currentDir || '/' }}</strong>
        </div>
        <div class="upload-box">
          <input type="file" @change="onFileChange" />
          <button @click="upload" :disabled="!uploadFile || uploading">
            {{ uploading ? '上传中...' : '上传到当前目录' }}
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

      <section class="content card">
        <div class="section-title">文件预览</div>
        <div class="status" v-if="error">{{ error }}</div>
        <div v-if="selectedFile">
          <div class="file-header">
            <div>
              <h2>{{ selectedFile.name }}</h2>
              <p class="subtitle">{{ selectedFile.path }}</p>
            </div>
            <div class="actions" v-if="fileType === 'markdown'">
              <button @click="toggleEdit">
                {{ isEditing ? '取消编辑' : '编辑内容' }}
              </button>
              <button v-if="isEditing" class="primary" @click="saveMarkdown" :disabled="saving">
                {{ saving ? '保存中...' : '保存' }}
              </button>
            </div>
          </div>

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

          <div v-else class="text-preview">
            <pre>{{ fileContent }}</pre>
          </div>
        </div>
        <div v-else class="empty">请选择左侧目录树中的文件进行预览。</div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import axios from 'axios';
import mermaid from 'mermaid';
import { marked } from 'marked';
import TreeNode from './components/TreeNode.vue';

const tree = ref(null);
const selectedFile = ref(null);
const selectedPath = ref('');
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

const renderer = new marked.Renderer();
const escapeHtml = (value) =>
  value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/\"/g, '&quot;')
    .replace(/'/g, '&#39;');

renderer.code = (code, info) => {
  if ((info || '').trim() === 'mermaid') {
    return `<div class="mermaid">${code}</div>`;
  }
  return `<pre><code>${escapeHtml(code)}</code></pre>`;
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
  selectedPath.value = node.path;
  if (node.type === 'dir') {
    currentDir.value = node.path;
    selectedFile.value = null;
    fileType.value = '';
    return;
  }

  selectedFile.value = node;
  currentDir.value = node.path.split('/').slice(0, -1).join('/');
  error.value = '';
  isEditing.value = false;
  try {
    const response = await axios.get('/api/file', {
      params: { path: node.path }
    });
    fileContent.value = response.data.content;
    fileType.value = response.data.type;
    if (fileType.value === 'image') {
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
  } catch (err) {
    error.value = '上传失败，请重试。';
  } finally {
    uploading.value = false;
    uploadFile.value = null;
  }
};

const toggleEdit = () => {
  isEditing.value = !isEditing.value;
  if (isEditing.value) {
    editContent.value = fileContent.value;
  }
};

const saveMarkdown = async () => {
  if (!selectedFile.value) return;
  saving.value = true;
  try {
    await axios.put(`/api/file?path=${encodeURIComponent(selectedFile.value.path)}`, editContent.value, {
      headers: { 'Content-Type': 'text/plain' }
    });
    fileContent.value = editContent.value;
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
  }
});

onMounted(fetchTree);
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 100%);
  padding: 32px;
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  color: #0f172a;
}

.hero {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.hero h1 {
  font-size: 2rem;
  margin: 0 0 4px 0;
}

.hero p {
  margin: 0;
  color: #475569;
}

.refresh {
  background: #4f46e5;
  border: none;
  color: white;
  padding: 10px 16px;
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
  grid-template-columns: 320px 1fr;
  gap: 24px;
}

.card {
  background: white;
  border-radius: 18px;
  padding: 20px;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.08);
}

.section-title {
  font-weight: 700;
  margin-bottom: 12px;
  font-size: 1.1rem;
  color: #1e293b;
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

.tree-container {
  max-height: 60vh;
  overflow: auto;
}

.content .file-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid #e2e8f0;
  padding-bottom: 12px;
  margin-bottom: 16px;
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

.markdown-area {
  line-height: 1.7;
}

.markdown h1,
.markdown h2,
.markdown h3 {
  margin-top: 1.2em;
}

.markdown pre {
  background: #0f172a;
  color: #f8fafc;
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
}
</style>
