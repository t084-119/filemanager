<template>
  <div class="page" :class="codeThemeClass">
    <main class="layout" :class="{ 'sidebar-hidden': !sidebarVisible }">
      <div v-if="sidebarVisible" class="sidebar-wrapper" :style="wrapperStyle">
        <section class="sidebar card" @mouseup="onSidebarResize" @mouseleave="onSidebarResize">
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
          <div class="permission-link">
            <a href="#" @click.prevent="showLoginModal = true" v-if="!isLoggedIn">获取权限</a>
          </div>
        </section>
        <button class="sidebar-toggle collapse" @click="toggleSidebar">&lt;</button>
      </div>
      <button v-else class="sidebar-toggle expand" @click="toggleSidebar">&gt;</button>

      <section class="content card">
        <div class="status" v-if="error">{{ error }}</div>
        <div v-if="permissionDenied">
          <div class="permission-denied">
            <div class="permission-icon">🔒</div>
            <div class="permission-text">暂无权限访问</div>
          </div>
        </div>
        <div v-else-if="selectedFile">
          <div class="preview-header">
            <div class="preview-title">
              <div class="section-title">{{ selectedFile ? selectedFile.name : '文件预览' }}</div>
              <h2 class="file-name" v-if="selectedFile" @click="toggleFileNameDisplay">
                {{ displayPath }}
              </h2>
            </div>
            <div class="preview-actions">
              <div class="theme-toggle" v-if="fileType === 'markdown' || fileType === 'json'">
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

          <div v-if="fileType === 'image'" class="image-preview">
            <img :src="imageUrl" :alt="selectedFile.name" />
          </div>

          <div v-else-if="fileType === 'markdown'" class="markdown-area">
            <textarea
              v-if="isEditing"
              v-model="editContent"
              class="editor"
              @keydown="handleTabKey"
            ></textarea>
            <div v-else ref="previewRef" class="markdown" v-html="renderedMarkdown"></div>
          </div>

          <div v-else-if="fileType === 'pdf'" class="pdf-preview">
            <iframe :src="imageUrl" title="PDF预览"></iframe>
          </div>

          <!-- <div v-else-if="fileType === 'json'" class="json-preview">
            <textarea v-if="isEditing" v-model="editContent" class="json-editor" @keydown="handleTabKey"></textarea>
            <pre v-else class="json-code">{{ fileContent }}</pre>
          </div> -->
          <div v-else-if="fileType === 'json'" class="json-preview">
           <textarea v-if="isEditing" v-model="editContent" class="editor" @keydown="handleTabKey"></textarea>
            <pre v-else class="json-code hljs" v-html="highlightedJson"></pre>
        </div>
          <div v-else class="text-preview">
            <textarea v-if="isEditing" v-model="editContent" class="editor" @keydown="handleTabKey"></textarea>
            <pre v-else>{{ fileContent }}</pre>
          </div>
        </div>
        <div v-else>
          <div class="preview-header">
            <div class="preview-title">
              <div class="section-title">文件预览</div>
            </div>
          </div>
          <div class="empty">请选择左侧目录树中的文件进行预览。</div>
        </div>
      </section>
    </main>

    <div v-if="showLoginModal" class="modal-overlay" @click="showLoginModal = false">
      <div class="modal" @click.stop>
        <div class="modal-header">
          <h3>获取权限</h3>
          <button class="close-btn" @click="showLoginModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>用户名</label>
            <input v-model="loginForm.username" type="text" placeholder="请输入用户名" />
          </div>
          <div class="form-group">
            <label>密码</label>
            <input v-model="loginForm.password" type="password" placeholder="请输入密码" />
          </div>
          <div class="form-actions">
            <button class="primary" @click="handleLogin" :disabled="loginLoading">
              {{ loginLoading ? '登录中...' : '登录' }}
            </button>
            <button class="secondary" @click="showLoginModal = false">取消</button>
          </div>
          <div v-if="loginError" class="error-message">{{ loginError }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue';
import axios from 'axios';
import hljs from 'highlight.js';
import mermaid from 'mermaid';
import { marked } from 'marked';
import TreeNode from './components/TreeNode.vue';
import './markdown.css';

axios.interceptors.response.use(
  response => {
    updateLastActivity();
    return response;
  },
  error => {
    if (error.response?.status === 401) {
      isLoggedIn.value = false;
      lastActivityTime.value = 0;
      localStorage.removeItem('username');
      localStorage.removeItem('isLoggedIn');
      localStorage.removeItem('lastActivityTime');
      error.value = '登录已过期，请重新登录';
    }
    return Promise.reject(error);
  }
);

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
const showFileName = ref(false);
const isLoggedIn = ref(false);
const showLoginModal = ref(false);
const loginForm = ref({ username: '', password: '' });
const loginLoading = ref(false);
const loginError = ref('');
const permissionDenied = ref(false);
const lastActivityTime = ref(0);
const SESSION_TTL = 5 * 1000 * 60;
const permissionExpired = ref(false);

const codeThemeClass = computed(() =>
  codeTheme.value === 'light' ? 'code-theme-light' : 'code-theme-dark'
);
const wrapperStyle = computed(() => ({
  '--sidebar-width': `${sidebarWidth.value}px`
}));
const isEditable = computed(() =>
  isLoggedIn.value && ['markdown', 'text', 'json'].includes(fileType.value)
);

const displayPath = computed(() => {
  if (!selectedFile.value) return '';
  const path = selectedFile.value.path;
  if (!showFileName.value && path.length > 25) {
    const start = path.substring(0, 12);
    const end = path.substring(path.length - 12);
    return `${start}...${end}`;
  }
  return path;
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
// marked.setOptions({ renderer, breaks: true });
marked.setOptions({
  renderer,
  breaks: true,
  smartypants: false
})

mermaid.initialize({ startOnLoad: false, theme: 'default' });

const renderedMarkdown = computed(() => {
  if (fileType.value !== 'markdown') return '';
  return marked.parse(fileContent.value || '');
});

const highlightedJson = computed(() => {
  if (fileType.value !== 'json') return '';
  const content = fileContent.value || '';
  if (!content.trim()) return '';
  return hljs.highlight(content, { language: 'json' }).value;
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
  updateLastActivity();
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
  showFileName.value = true;
  error.value = '';
  permissionDenied.value = false;
  // permissionExpired.value = false;
  isEditing.value = false;
  try {
    const headers = isLoggedIn.value ? { 'X-Session-Token': localStorage.getItem('username') || '' } : {};
    const response = await axios.get('/api/file', {
      params: { path: node.path },
      headers
    });
    fileContent.value = response.data.content;
    fileType.value = response.data.type;
    if (fileType.value === 'json') {
      formatJsonContent(fileContent.value, 'display');
    }
    if (fileType.value === 'image' || fileType.value === 'pdf') {
      try {
        const rawResponse = await axios.get('/api/raw', {
          params: { path: node.path },
          headers,
          responseType: 'blob'
        });
        imageUrl.value = URL.createObjectURL(rawResponse.data);
      } catch (rawErr) {
        handleAuthError(rawErr);
        if (rawErr.response?.status === 403) {
          permissionDenied.value = true;
          // permissionExpired.value = true;
          isLoggedIn.value = false;
          localStorage.setItem('isLoggedIn', 'false');
          error.value = '暂无权限访问';
          selectedFile.value = node;
          fileType.value = '';
        } else if (rawErr.response?.status !== 401) {
          error.value = '无法加载文件内容。';
        }
      }
    }
  } catch (err) {
    handleAuthError(err);
    if (err.response?.status === 403) {
      permissionDenied.value = true;
      // permissionExpired.value = true;
      isLoggedIn.value = false;
      localStorage.setItem('isLoggedIn', 'false');
      error.value = '暂无权限访问';
      selectedFile.value = node;
      fileType.value = '';
    } else if (err.response?.status !== 401) {
      error.value = '无法加载文件内容。';
    }
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
  if (!isLoggedIn.value) {
    error.value = '请先登录以创建文件夹';
    return;
  }
  const name = window.prompt('请输入新建文件夹名称');
  if (!name) return;
  const parent = selectedNode.value?.type === 'dir' ? selectedNode.value.path : currentDir.value;
  try {
    await axios.post('/api/create', {
      parent,
      name,
      type: 'dir'
    }, {
      headers: { 'X-Session-Token': localStorage.getItem('username') || '' }
    });
    await fetchTree();
  } catch (err) {
    handleAuthError(err);
    if (err.response?.status !== 401) {
      error.value = err?.response?.data?.error
        ? `创建失败：${err.response.data.error}`
        : '创建失败，请重试。';
    }
  }
};

const createFile = async () => {
  if (!isLoggedIn.value) {
    error.value = '请先登录以创建文件';
    return;
  }
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
    }, {
      headers: { 'X-Session-Token': localStorage.getItem('username') || '' }
    });
    await fetchTree();
  } catch (err) {
    handleAuthError(err);
    if (err.response?.status !== 401) {
      error.value = err?.response?.data?.error
        ? `创建失败：${err.response.data.error}`
        : '创建失败，请重试。';
    }
  }
};

const deleteSelected = async () => {
  if (!selectedNode.value) return;
  if (!isLoggedIn.value) {
    error.value = '请先登录以删除文件';
    return;
  }
  if (selectedNode.value.path === '') {
    error.value = '根目录不可删除。';
    return;
  }
  if (!window.confirm(`确定要删除 ${selectedNode.value.name} 吗？`)) return;
  try {
    await axios.delete('/api/file', { 
      params: { path: selectedNode.value.path },
      headers: { 'X-Session-Token': localStorage.getItem('username') || '' }
    });
    selectedNode.value = null;
    selectedFile.value = null;
    selectedPath.value = '';
    fileType.value = '';
    await fetchTree();
  } catch (err) {
    handleAuthError(err);
    if (err.response?.status !== 401) {
      error.value = err?.response?.data?.error
        ? `删除失败：${err.response.data.error}`
        : '删除失败，请重试。';
    }
  }
};

const toggleEdit = () => {
  updateLastActivity();
  isEditing.value = !isEditing.value;
  if (isEditing.value) {
    editContent.value = fileContent.value;
  }
};

const formatJsonContent = (content, target = 'display') => {
  try {
    const parsed = JSON.parse(content);
    const formatted = JSON.stringify(parsed, null, 2);
    if (target === 'edit') {
      editContent.value = formatted;
    } else {
      fileContent.value = formatted;
    }
    return true;
  } catch (err) {
    return false;
  }
};

const saveFile = async () => {
  if (!selectedFile.value) return;
  if (!isLoggedIn.value) {
    error.value = '请先登录以保存文件';
    return;
  }
  saving.value = true;
  try {
    await axios.put(`/api/file?path=${encodeURIComponent(selectedFile.value.path)}`, editContent.value, {
      headers: { 
        'Content-Type': 'text/plain',
        'X-Session-Token': localStorage.getItem('username') || ''
      }
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
    handleAuthError(err);
    if (err.response?.status !== 401) {
      error.value = err?.response?.data?.error
        ? `保存失败：${err.response.data.error}`
        : '保存失败，请重试。';
    }
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

const toggleTheme = () => {
  codeTheme.value = codeTheme.value === 'light' ? 'dark' : 'light';
};

const updateSidebarMetrics = () => {
  const element = document.querySelector('.sidebar');
  if (!element) return;
  const rect = element.getBoundingClientRect();
  sidebarWidth.value = rect.width;
  sidebarHeight.value = rect.height;
};

const toggleSidebar = () => {
  sidebarVisible.value = !sidebarVisible.value;
  if (sidebarVisible.value) {
    nextTick(() => updateSidebarMetrics());
  }
};

const onSidebarResize = () => {
  updateSidebarMetrics();
};

const toggleFileNameDisplay = () => {
  showFileName.value = !showFileName.value;
};

const handleTabKey = (e) => {
  if (e.key === 'Tab') {
    e.preventDefault();
    const textarea = e.target;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const value = textarea.value;
    
    textarea.value = value.substring(0, start) + '  ' + value.substring(end);
    textarea.selectionStart = textarea.selectionEnd = start + 2;
    editContent.value = textarea.value;
  }
};

const handleAuthError = (err) => {
  if (err.response?.status === 401) {
    isLoggedIn.value = false;
    lastActivityTime.value = 0;
    localStorage.removeItem('username');
    localStorage.removeItem('isLoggedIn');
    localStorage.removeItem('lastActivityTime');
    error.value = '登录已过期，请重新登录';
  }
};

const updateLastActivity = () => {
  if (isLoggedIn.value) {
    const now = Date.now();
    if (now - lastActivityTime.value > 1000) {
      lastActivityTime.value = now;
      localStorage.setItem('lastActivityTime', now.toString());
    }
  }
};

const handleLogin = async () => {
  if (!loginForm.value.username || !loginForm.value.password) {
    loginError.value = '请输入用户名和密码';
    return;
  }
  
  loginLoading.value = true;
  loginError.value = '';
  
  try {
    const response = await axios.post('/api/login', {
      username: loginForm.value.username,
      password: loginForm.value.password
    });
    
    if (response.data.status === 'success') {
      isLoggedIn.value = true;
      permissionExpired.value = false;
      lastActivityTime.value = Date.now();
      // console.log(`[Login] Logged in at ${lastActivityTime.value}`);
      localStorage.setItem('username', response.data.username);
      localStorage.setItem('isLoggedIn', 'true');
      localStorage.setItem('lastActivityTime', Date.now().toString());
      showLoginModal.value = false;
      loginForm.value = { username: '', password: '' };
      error.value = '';
    }
  } catch (err) {
    loginError.value = err.response?.data?.error || '登录失败，请重试';
  } finally {
    loginLoading.value = false;
  }
};

const checkLoginStatus = async () => {
  const storedUsername = localStorage.getItem('username');
  const storedLoggedIn = localStorage.getItem('isLoggedIn');
  const storedLastActivityTime = localStorage.getItem('lastActivityTime');
  
  if (storedUsername && storedLoggedIn === 'true' && storedLastActivityTime) {
    lastActivityTime.value = parseInt(storedLastActivityTime);
    
    try {
      const response = await axios.get('/api/tree');
      // isLoggedIn.value = true;
    } catch (err) {
      if (err.response?.status === 401) {
        isLoggedIn.value = false;
        lastActivityTime.value = 0;
        localStorage.removeItem('username');
        localStorage.removeItem('isLoggedIn');
        localStorage.removeItem('lastActivityTime');
      } 
    }
  }
  
  const urlParams = new URLSearchParams(window.location.search);
  const urlUsername = urlParams.get('username');
  const urlPasswd = urlParams.get('passwd');
  
  if (urlUsername && urlPasswd) {
    loginForm.value = { username: urlUsername, password: urlPasswd };
    handleLogin();
    window.history.replaceState({}, document.title, window.location.pathname);
  }
};

const checkLoginExpiration = () => {
  if (isLoggedIn.value && lastActivityTime.value > 0) {
    const now = Date.now();
    const elapsed = now - lastActivityTime.value;
    
    // console.log(`[Login Check] Inactive: ${elapsed}ms, TTL: ${SESSION_TTL}ms, isLoggedIn: ${isLoggedIn.value}`);
    
    if (elapsed >= SESSION_TTL) {
      if(isLoggedIn.value === true){
        // console.log('[Login Check] Session expired due to inactivity, logging out...');
        isLoggedIn.value = false;
        localStorage.removeItem('username');
        localStorage.removeItem('isLoggedIn');
        localStorage.removeItem('lastActivityTime');
        lastActivityTime.value = 0;
      }
    }
  }
};

const restoreLoginState = () => {
  const storedLoggedIn = localStorage.getItem('isLoggedIn');
  if (storedLoggedIn !== null) {
    // console.log(`[Restore Login State] Stored isLoggedIn: ${storedLoggedIn}`);
    isLoggedIn.value = storedLoggedIn === 'true';
  }
};

watch(isLoggedIn, (newValue) => {
  if(newValue === false){
    window.location.reload();
    // console.log('[Watch] Logged out');
  }
});

onMounted(() => {
  restoreLoginState();
  fetchTree();
  nextTick(() => updateSidebarMetrics());
  
  setInterval(() => {
    checkLoginExpiration();
  }, 5000);
  
  const activityEvents = [
    'click',
    'keydown',
    'keyup',
    'mousedown',
    'mouseup',
    'mousemove',
    'scroll',
    'touchstart',
    'touchend',
    'touchmove',
    'input',
    'change',
    'paste'
  ];
  
  activityEvents.forEach(event => {
    document.addEventListener(event, updateLastActivity, { passive: true });
  });
  // document.addEventListener('click', () => {
  //   console.log(isLoggedIn.value);
  // });
});

onUnmounted(() => {
  const activityEvents = [
    'click',
    'keydown',
    'keyup',
    'mousedown',
    'mouseup',
    'mousemove',
    'scroll',
    'touchstart',
    'touchend',
    'touchmove',
    'input',
    'change',
    'paste'
  ];
  
  activityEvents.forEach(event => {
    document.removeEventListener(event, updateLastActivity);
  });
});
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: radial-gradient(circle at top, #eef2ff 0%, #f8fafc 45%, #f1f5f9 100%);
  padding: 32px;
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  color: #0f172a;
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
  align-items: start;
  position: relative;
  padding-left: 0;
}

.layout.sidebar-hidden {
  grid-template-columns: 1fr;
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
  max-height: calc(100vh - 40px);
  overflow: auto;
  resize: both;
  min-width: 260px;
  max-width: 520px;
  width: 320px;
  min-height: 360px;
  display: flex;
  flex-direction: column;
  z-index: 100;
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
  font-size: 1.6rem;
  color: #0f172a;
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
  flex-direction: column;
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
  min-height: 140px;
  max-height: calc(100vh - 420px);
  overflow: auto;
}

.content {
  position: relative;
  z-index: 1;
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
}

.preview-title {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.file-name {
  margin: 0;
  font-size: 0.85rem;
  color: #94a3b8;
  cursor: pointer;
  user-select: none;
}

.file-name:hover {
  color: #4f46e5;
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

.image-preview {
  max-height: calc(100vh - 180px);
  overflow-y: auto;
  padding-right: 8px;
}

.image-preview::-webkit-scrollbar {
  width: 8px;
}

.image-preview::-webkit-scrollbar-track {
  background: #f1f5f9;
  border-radius: 4px;
}

.image-preview::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 4px;
}

.image-preview::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
}

.pdf-preview iframe {
  width: 100%;
  height: calc(100vh - 180px);
  border: none;
  border-radius: 12px;
  background: #f8fafc;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.12);
}

.pdf-preview {
  max-height: calc(100vh - 180px);
  overflow-y: auto;
  padding-right: 8px;
}

.pdf-preview::-webkit-scrollbar {
  width: 8px;
}

.pdf-preview::-webkit-scrollbar-track {
  background: #f1f5f9;
  border-radius: 4px;
}

.pdf-preview::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 4px;
}

.pdf-preview::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
}

/* .markdown-area {
  line-height: 1.7;
  max-height: calc(100vh - 180px);
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 8px;
}

.markdown-area::-webkit-scrollbar {
  width: 8px;
}

.markdown-area::-webkit-scrollbar-track {
  background: #f1f5f9;
  border-radius: 4px;
}

.markdown-area::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 4px;
}

.markdown-area::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
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

.markdown table {
  width: 100%;
  border-collapse: collapse;
  margin: 12px 0 16px;
  font-size: 0.95rem;
}

.markdown th,
.markdown td {
  border: 1px solid rgba(148, 163, 184, 0.5);
  padding: 10px 12px;
  text-align: center;
}

.markdown thead {
  background: rgba(248, 250, 252, 0.9);
}

.markdown pre {
  background: transparent;
  padding: 12px;
  border-radius: 10px;
  overflow: auto;
} */

.editor {
  width: 100%;
  min-height: 320px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 16px;
  font-family: 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 15px;
  line-height: 1.6;
  color: #334155;
  background: #f8fafc;
  resize: vertical;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.editor:focus {
  outline: none;
  border-color: #4f46e5;
  box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.1);
}

.json-editor {
  width: 100%;
  min-height: 320px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 12px;
  padding: 16px;
  font-family: 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.95rem;
  line-height: 1.6;
  color: #e2e8f0;
  background: #0f172a;
  resize: vertical;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.json-editor:focus {
  outline: none;
  border-color: #4f46e5;
  box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.3);
}

.text-preview pre {
  background: #f1f5f9;
  padding: 12px;
  border-radius: 10px;
}

.text-preview {
  max-height: calc(100vh - 180px);
  overflow-y: auto;
  padding-right: 8px;
}

.text-preview::-webkit-scrollbar {
  width: 8px;
}

.text-preview::-webkit-scrollbar-track {
  background: #f1f5f9;
  border-radius: 4px;
}

.text-preview::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 4px;
}

.text-preview::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
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
}

.permission-link {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid rgba(148, 163, 184, 0.2);
  text-align: center;
}

.permission-link a {
  color: #3b82f6;
  text-decoration: underline;
  font-size: 0.85rem;
  cursor: pointer;
  transition: color 0.2s ease;
}

.permission-link a:hover {
  color: #2563eb;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: white;
  border-radius: 18px;
  padding: 24px;
  width: 100%;
  max-width: 400px;
  box-shadow: 0 20px 45px rgba(15, 23, 42, 0.15);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  color: #0f172a;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #64748b;
  padding: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  transition: background 0.2s ease;
}

.close-btn:hover {
  background: rgba(148, 163, 184, 0.1);
}

.modal-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-group label {
  font-size: 0.9rem;
  font-weight: 500;
  color: #475569;
}

.form-group input {
  padding: 10px 12px;
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 8px;
  font-size: 0.95rem;
  transition: border-color 0.2s ease;
}

.form-group input:focus {
  outline: none;
  border-color: #4f46e5;
}

.form-actions {
  display: flex;
  gap: 12px;
  margin-top: 8px;
}

.form-actions button {
  flex: 1;
  padding: 10px 16px;
  border: none;
  border-radius: 8px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.form-actions button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.error-message {
  color: #dc2626;
  font-size: 0.9rem;
  text-align: center;
  padding: 8px;
  background: rgba(220, 38, 38, 0.1);
  border-radius: 8px;
}

.permission-denied {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
}

.permission-icon {
  font-size: 4rem;
  margin-bottom: 16px;
}

.permission-text {
  font-size: 1.5rem;
  font-weight: 600;
  color: #0f172a;
  margin-bottom: 8px;
}

.permission-subtext {
  font-size: 0.95rem;
  color: #64748b;
}
</style>

