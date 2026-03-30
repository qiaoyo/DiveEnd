// DiveEnd Frontend JavaScript

// App state
let state = {
  currentFolderId: null,
  currentPaperId: null,
  currentPaper: null,
  folders: [],
  papers: [],
  searchResults: [],
  selectedSearchResults: new Set(),
  classifications: [],
};

// DOM elements
const elements = {
  // Tabs
  tabBtns: document.querySelectorAll('.tab-btn'),
  tabContents: document.querySelectorAll('.tab-content'),

  // Config
  llmProvider: document.getElementById('llm-provider'),
  llmApiKey: document.getElementById('llm-api-key'),
  llmModel: document.getElementById('llm-model'),
  llmBaseUrl: document.getElementById('llm-base-url'),
  baiduEnabled: document.getElementById('baidu-enabled'),
  baiduAk: document.getElementById('baidu-ak'),
  baiduSk: document.getElementById('baidu-sk'),
  btnSaveConfig: document.getElementById('btn-save-config'),
  configStatus: document.getElementById('config-status'),
  btnSyncNow: document.getElementById('btn-sync-now'),

  // DeepStart
  deepstartQuery: document.getElementById('deepstart-query'),
  btnDeepstartSearch: document.getElementById('btn-deepstart-search'),
  deepstartLoading: document.getElementById('deepstart-loading'),
  deepstartResults: document.getElementById('deepstart-results'),
  searchResultsList: document.getElementById('search-results-list'),
  classificationContent: document.getElementById('classification-content'),
  importFolder: document.getElementById('import-folder'),
  btnCreateFolder: document.getElementById('btn-create-folder'),
  btnImportSelected: document.getElementById('btn-import-selected'),

  // DeepRead
  deepreadNoSelection: document.getElementById('deepread-no-selection'),
  deepreadContent: document.getElementById('deepread-content'),
  paperTitle: document.getElementById('paper-title'),
  paperAuthors: document.getElementById('paper-authors'),
  paperAbstract: document.getElementById('paper-abstract'),
  paperUrl: document.getElementById('paper-url'),
  pasteText: document.getElementById('paste-text'),
  sectionName: document.getElementById('section-name'),
  btnDeepreadTranslate: document.getElementById('btn-deepread-translate'),
  translatedSections: document.getElementById('translated-sections'),

  // Right panel
  folderSelect: document.getElementById('folder-select'),
  paperList: document.getElementById('paper-list'),

  // Translation sidebar
  translationContent: document.getElementById('translation-content'),
};

// API helpers
async function apiGet(endpoint) {
  const res = await fetch(endpoint);
  return res.json();
}

async function apiPost(endpoint, data) {
  const res = await fetch(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  return res.json();
}

// Tab switching
elements.tabBtns.forEach(btn => {
  btn.addEventListener('click', () => {
    const tabName = btn.dataset.tab;

    elements.tabBtns.forEach(b => b.classList.remove('active'));
    elements.tabContents.forEach(c => c.classList.remove('active'));

    btn.classList.add('active');
    document.getElementById(`${tabName}-tab`).classList.add('active');
  });
});

// Load config
async function loadConfig() {
  const config = await apiGet('/api/config');
  // Config doesn't return secrets for security
}

// Save config
elements.btnSaveConfig.addEventListener('click', async () => {
  const data = {
    llm_provider: elements.llmProvider.value,
    llm_api_key: elements.llmApiKey.value,
    llm_model: elements.llmModel.value,
    llm_base_url: elements.llmBaseUrl.value,
    baidu_enabled: elements.baiduEnabled.checked,
    baidu_ak: elements.baiduAk.value,
    baidu_sk: elements.baiduSk.value,
  };

  const res = await apiPost('/api/config/save', data);
  if (res.error) {
    elements.configStatus.textContent = `错误: ${res.error}`;
    elements.configStatus.style.color = 'red';
  } else {
    elements.configStatus.textContent = '✓ 配置已保存';
    elements.configStatus.style.color = 'green';
  }
});

elements.btnSyncNow.addEventListener('click', async () => {
  const res = await apiPost('/api/sync/baidu', {});
  alert(res.error ? `同步失败: ${res.error}` : '同步完成！');
});

// Load folders
async function loadFolders() {
  const res = await apiGet('/api/folders');
  state.folders = res.folders || [];

  // Update folder select
  elements.importFolder.innerHTML = '<option value="">选择目标文件夹...</option>';
  elements.folderSelect.innerHTML = '<option value="">全部</option>';

  res.folders.forEach(f => {
    const opt1 = document.createElement('option');
    opt1.value = f.id;
    opt1.textContent = f.name;
    elements.importFolder.appendChild(opt1);

    const opt2 = document.createElement('option');
    opt2.value = f.id;
    opt2.textContent = f.name;
    elements.folderSelect.appendChild(opt2);
  });
}

elements.folderSelect.addEventListener('change', async () => {
  const folderId = elements.folderSelect.value;
  if (folderId) {
    state.currentFolderId = parseInt(folderId);
    await loadPapers(state.currentFolderId);
  } else {
    state.currentFolderId = null;
    elements.paperList.innerHTML = '';
  }
});

elements.btnCreateFolder.addEventListener('click', async () => {
  const name = prompt('请输入文件夹名称:');
  if (!name) return;
  const description = prompt('请输入描述(可选):') || '';

  const res = await apiPost('/api/folders/create', { name, description });
  if (res.id) {
    await loadFolders();
    alert('文件夹创建成功！');
  }
});

// Load papers
async function loadPapers(folderId) {
  const res = await apiGet(`/api/papers?folder_id=${folderId}`);
  state.papers = res.papers || [];
  renderPaperList();
}

function renderPaperList() {
  elements.paperList.innerHTML = '';

  state.papers.forEach(paper => {
    const item = document.createElement('div');
    item.className = `paper-item ${state.currentPaperId === paper.id ? 'selected' : ''}`;
    item.innerHTML = `
      <h4>${escapeHtml(paper.title)}</h4>
      <div class="authors">${escapeHtml(paper.authors || '')}</div>
    `;
    item.addEventListener('click', () => {
      selectPaper(paper);
    });
    elements.paperList.appendChild(item);
  });
}

async function selectPaper(paper) {
  // Switch to deepread tab
  elements.tabBtns.forEach(b => b.dataset.tab === 'deepread' && b.click());

  state.currentPaperId = paper.id;
  state.currentPaper = paper;

  elements.deepreadNoSelection.classList.add('hidden');
  elements.deepreadContent.classList.remove('hidden');

  elements.paperTitle.textContent = paper.title;
  elements.paperAuthors.textContent = paper.authors || '';
  elements.paperAbstract.textContent = paper.abstract || '';
  elements.paperUrl.href = paper.url || '#';

  // Load existing translations
  await loadTranslations(paper.id);

  renderPaperList();
}

// DeepStart search
elements.btnDeepstartSearch.addEventListener('click', async () => {
  const query = elements.deepstartQuery.value.trim();
  if (!query) {
    alert('请输入查询内容');
    return;
  }

  elements.deepstartLoading.classList.remove('hidden');
  elements.deepstartResults.classList.add('hidden');

  try {
    const res = await apiPost('/api/search/deepstart', { query });
    state.searchResults = res.results || [];
    state.classifications = res.classification || [];
    state.selectedSearchResults.clear();

    renderSearchResults();
    renderClassification();

    elements.deepstartResults.classList.remove('hidden');
  } catch (e) {
    alert(`搜索失败: ${e.message}`);
  } finally {
    elements.deepstartLoading.classList.add('hidden');
  }
});

function renderSearchResults() {
  elements.searchResultsList.innerHTML = '';

  state.searchResults.forEach((result, index) => {
    const item = document.createElement('div');
    item.className = 'search-result-item';

    const isSelected = state.selectedSearchResults.has(index);
    item.innerHTML = `
      <h4>
        <input type="checkbox" ${isSelected ? 'checked' : ''}>
        ${escapeHtml(result.title)}
      </h4>
      <div class="authors">${escapeHtml(result.authors || '')} ${result.year ? `(${result.year})` : ''} - ${escapeHtml(result.venue || '')}</div>
      <div class="abstract">${escapeHtml(result.abstract || '')}</div>
    `;

    const checkbox = item.querySelector('input');
    checkbox.addEventListener('change', () => {
      if (checkbox.checked) {
        state.selectedSearchResults.add(index);
        item.classList.add('selected');
      } else {
        state.selectedSearchResults.delete(index);
        item.classList.remove('selected');
      }
    });

    if (isSelected) {
      item.classList.add('selected');
    }

    elements.searchResultsList.appendChild(item);
  });
}

function renderClassification() {
  if (!state.classifications.length) {
    elements.classificationContent.innerHTML = '<p>暂无分类结果</p>';
    return;
  }

  let html = '';
  state.classifications.forEach(c => {
    const tags = c.categories.map(cat => `<span class="category-tag">${escapeHtml(cat)}</span>`).join('');
    html += `
      <div class="classification-item">
        <h4>${escapeHtml(c.title)}</h4>
        <div class="categories">${tags}</div>
        <div class="relevance-score">相关度: ${c.relevance}/10</div>
        <p style="margin-top: 8px; font-size: 14px;">${escapeHtml(c.reasoning)}</p>
      </div>
    `;
  });
  elements.classificationContent.innerHTML = html;
}

elements.btnImportSelected.addEventListener('click', async () => {
  const folderId = parseInt(elements.importFolder.value);
  if (!folderId) {
    alert('请先选择目标文件夹');
    return;
  }

  if (state.selectedSearchResults.size === 0) {
    alert('请至少选择一篇论文');
    return;
  }

  const papersToImport = Array.from(state.selectedSearchResults).map(i => {
    const r = state.searchResults[i];
    return {
      title: r.title,
      authors: r.authors,
      abstract: r.abstract,
      url: r.url,
      category: (state.classifications[i] || {}).categories ? (state.classifications[i].categories[0] || '') : '',
    };
  });

  const res = await apiPost('/api/paper/import', {
    papers: papersToImport,
    folder_id: folderId,
  });

  alert(`成功导入 ${papersToImport.length} 篇论文！`);

  // Reload papers
  await loadPapers(folderId);
});

// DeepRead translation
elements.btnDeepreadTranslate.addEventListener('click', async () => {
  if (!state.currentPaperId) {
    alert('请先选择论文');
    return;
  }

  const text = elements.pasteText.value.trim();
  const section = elements.sectionName.value.trim() || 'Untitled';

  if (!text) {
    alert('请粘贴论文内容');
    return;
  }

  elements.btnDeepreadTranslate.disabled = true;
  elements.btnDeepreadTranslate.textContent = '翻译中...';

  try {
    const res = await apiPost('/api/paper/deepread', {
      paper_id: state.currentPaperId,
      section: section,
      text: text,
    });

    // Add to translated sections
    addTranslatedSection(section, text, res.translation, res.summary);

    // Clear input
    elements.pasteText.value = '';
    elements.sectionName.value = '';
  } catch (e) {
    alert(`翻译失败: ${e.message}`);
  } finally {
    elements.btnDeepreadTranslate.disabled = false;
    elements.btnDeepreadTranslate.textContent = '一键AI翻译 ✨';
  }
});

function addTranslatedSection(section, original, translation, summary) {
  const div = document.createElement('div');
  div.className = 'translated-section';
  div.innerHTML = `
    <div class="section-title">${escapeHtml(section)}</div>
    <div class="original">${escapeHtml(original)}</div>
    <div class="translation">${escapeHtml(translation).replace(/\n/g, '<br>')}</div>
    ${summary ? `<div class="summary"><strong>要点总结:</strong> ${escapeHtml(summary)}</div>` : ''}
  `;
  elements.translatedSections.appendChild(div);
}

async function loadTranslations(paperId) {
  elements.translatedSections.innerHTML = '';
  const res = await apiGet(`/api/paper/translations?paper_id=${paperId}`);
  if (res.translations) {
    res.translations.forEach(t => {
      addTranslatedSection(t.section, t.original_text, t.translated_text, t.summary);
    });
  }
}

// Helper: escape HTML
function escapeHtml(text) {
  if (!text) return '';
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  loadConfig();
  loadFolders();
});
