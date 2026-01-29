// ============================================
// AI Multi-Agent Debate - Wails Desktop Version
// ============================================

// State
let isConnected = true; // Always connected in desktop app
let isDebateRunning = false;
let isAutoMode = false;
let agents = [];
let agentConfigs = [];
let currentStreamingMessage = null;
let selectedAgents = new Set();
let autoAgentIndex = 0;
let hiddenAgents = new Set();
let editingAgentId = null;
let pendingTurn = false;
let currentStreamingContent = '';
let hasMessages = false;

// DOM Elements
const topicInput = document.getElementById('topicInput');
const startBtn = document.getElementById('startBtn');
const continueBtn = document.getElementById('continueBtn');
const stopBtn = document.getElementById('stopBtn');
const autoBtn = document.getElementById('autoBtn');
const nextAgentBtn = document.getElementById('nextAgentBtn');
const exportMdBtn = document.getElementById('exportMdBtn');
const importMdBtn = document.getElementById('importMdBtn');
const importMdInput = document.getElementById('importMdInput');
const modeSelect = document.getElementById('modeSelect');
const messagesContainer = document.getElementById('messages');
const agentsList = document.getElementById('agentsList');
const statusIndicator = document.querySelector('.status-indicator');
const statusText = document.getElementById('statusText');
const scrollToBottomBtn = document.getElementById('scrollToBottomBtn');

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    // Setup Wails event listeners
    setupWailsEvents();

    // Load initial data
    await loadAgents();
    loadHiddenAgents();
    setupEventListeners();
    setupSidebarResize();
    setupAgentManager();
    setupHelpModal();
    setupScrollButton();

    // Set status to connected (always connected in desktop app)
    updateStatus('online', 'Sẵn sàng');

    // Check if marked.js is loaded
    if (typeof marked === 'undefined') {
        console.warn('marked.js not loaded, using fallback markdown renderer');
    } else {
        console.log('marked.js loaded successfully');
    }
});

// ============================================
// WAILS EVENT LISTENERS
// ============================================
function setupWailsEvents() {
    // Agent started speaking
    window.runtime.EventsOn('agent:start', (data) => {
        currentStreamingMessage = createMessage(data);
        updateStatus('processing', `${data.agent_name} đang suy nghĩ...`);
    });

    // Agent streaming content
    window.runtime.EventsOn('agent:chunk', (data) => {
        if (currentStreamingMessage) {
            appendToMessage(currentStreamingMessage, data.content);
        }
    });

    // Agent finished speaking
    window.runtime.EventsOn('agent:end', (data) => {
        if (currentStreamingMessage) {
            finalizeMessage(currentStreamingMessage);
            currentStreamingMessage = null;
        }
        updateStatus('online', 'Sẵn sàng');

        // Check if we should stop after this message
        if (window.stopAfterCurrentMessage) {
            window.stopAfterCurrentMessage = false;
            window.go.main.App.StopDebate();
            isDebateRunning = false;
            updateControls();
            return;
        }

        // If auto mode, trigger next turn
        if (isAutoMode && isDebateRunning && !pendingTurn) {
            pendingTurn = true;
            setTimeout(triggerNextTurn, 2000);
        }
    });

    // Agent error
    window.runtime.EventsOn('agent:error', (data) => {
        console.error('Agent error:', data.error);
        let errorMsg = data.error || 'Lỗi không xác định';
        if (errorMsg.includes('429')) {
            errorMsg = 'Hết quota API. Vui lòng kiểm tra tài khoản provider.';
        } else if (errorMsg.includes('401') || errorMsg.includes('403')) {
            errorMsg = 'Lỗi xác thực API key. Vui lòng kiểm tra cấu hình.';
        } else if (errorMsg.includes('timeout')) {
            errorMsg = 'Hết thời gian chờ phản hồi từ API.';
        }
        updateStatus('offline', `Lỗi: ${errorMsg}`);
        addSystemMessage(`⚠️ Lỗi: ${errorMsg}`);
        if (currentStreamingMessage) {
            finalizeMessage(currentStreamingMessage);
            currentStreamingMessage = null;
        }
    });

    // Debate started
    window.runtime.EventsOn('debate:started', (data) => {
        isDebateRunning = true;
        updateControls();
        clearWelcomeMessage();
    });

    // Debate stopped
    window.runtime.EventsOn('debate:stopped', () => {
        isDebateRunning = false;
        stopAutoMode();
        updateControls();
    });

    // Debate reset
    window.runtime.EventsOn('debate:reset', () => {
        isDebateRunning = false;
        hasMessages = false;
        stopAutoMode();
        messagesContainer.innerHTML = `
            <div class="welcome-message">
                <h2>Chào mừng đến với AI Multi-Agent Debate!</h2>
                <p>Nhập một chủ đề và nhấn "Bắt đầu" để các AI agent bắt đầu thảo luận.</p>
            </div>
        `;
        updateControls();
    });

    // Mode changed
    window.runtime.EventsOn('mode:changed', (data) => {
        modeSelect.value = data.mode;
    });

    // Topic changed (continued)
    window.runtime.EventsOn('debate:continued', (data) => {
        isDebateRunning = true;
        updateControls();
        addSystemMessage(`Chuyển sang chủ đề mới: ${data.topic}`);
    });

    // Agents reloaded
    window.runtime.EventsOn('agents:reloaded', () => {
        loadAgents();
        loadAgentConfigs();
    });
}

// ============================================
// LOAD AGENTS (Using Wails bindings)
// ============================================
async function loadAgents() {
    try {
        agents = await window.go.main.App.GetAgents();
        renderAgents();
    } catch (error) {
        console.error('Failed to load agents:', error);
    }
}

// Render Agents as clickable buttons with checkboxes
function renderAgents() {
    const visibleAgents = agents.filter(a => !hiddenAgents.has(a.id));

    if (selectedAgents.size === 0 && visibleAgents.length > 0) {
        visibleAgents.forEach(agent => selectedAgents.add(agent.id));
    }

    hiddenAgents.forEach(id => selectedAgents.delete(id));

    const selectedList = visibleAgents.filter(a => selectedAgents.has(a.id));
    const orderMap = new Map();
    selectedList.forEach((agent, idx) => orderMap.set(agent.id, idx + 1));

    agentsList.innerHTML = visibleAgents.map((agent, index) => {
        const order = orderMap.get(agent.id);
        const orderDisplay = order ? `<span class="agent-order">${order}</span>` : `<span class="agent-order agent-order-hidden">-</span>`;

        return `
        <div class="agent-item" draggable="true" data-index="${index}" data-agent-id="${agent.id}">
            <label class="agent-checkbox-wrapper">
                <input type="checkbox" class="agent-checkbox" data-agent-id="${agent.id}" ${selectedAgents.has(agent.id) ? 'checked' : ''}>
                <span class="checkbox-custom"></span>
            </label>
            ${orderDisplay}
            <button class="agent-btn" data-agent-id="${agent.id}" style="border-left-color: ${agent.color}" ${!isDebateRunning ? 'disabled' : ''}>
                <div class="agent-avatar" style="background: ${agent.color}">
                    ${agent.name.charAt(0)}
                </div>
                <div class="agent-info">
                    <h4>${agent.name}</h4>
                    <p>${agent.role}</p>
                </div>
            </button>
        </div>
    `}).join('');

    // Add click handlers
    document.querySelectorAll('.agent-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const agentId = btn.dataset.agentId;
            triggerAgentTurn(agentId);
        });
    });

    // Add checkbox handlers
    document.querySelectorAll('.agent-checkbox').forEach(checkbox => {
        checkbox.addEventListener('change', (e) => {
            const agentId = e.target.dataset.agentId;
            if (e.target.checked) {
                selectedAgents.add(agentId);
            } else {
                selectedAgents.delete(agentId);
            }
            renderAgents();
        });
    });

    setupDragDrop();
}

// Drag and Drop
let draggedItem = null;
let draggedIndex = null;

function setupDragDrop() {
    const items = document.querySelectorAll('.agent-item');

    items.forEach(item => {
        item.addEventListener('dragstart', handleDragStart);
        item.addEventListener('dragend', handleDragEnd);
        item.addEventListener('dragover', handleDragOver);
        item.addEventListener('dragenter', handleDragEnter);
        item.addEventListener('dragleave', handleDragLeave);
        item.addEventListener('drop', handleDrop);
    });
}

function handleDragStart(e) {
    draggedItem = this;
    draggedIndex = parseInt(this.dataset.index);
    this.classList.add('dragging');
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', draggedIndex);
}

function handleDragEnd() {
    this.classList.remove('dragging');
    document.querySelectorAll('.agent-item').forEach(item => {
        item.classList.remove('drag-over', 'drag-over-top');
    });
    draggedItem = null;
    draggedIndex = null;
}

function handleDragOver(e) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
}

function handleDragEnter(e) {
    e.preventDefault();
    if (this !== draggedItem) {
        this.classList.add('drag-over-top');
    }
}

function handleDragLeave() {
    this.classList.remove('drag-over', 'drag-over-top');
}

function handleDrop(e) {
    e.preventDefault();
    if (this !== draggedItem) {
        const dropIndex = parseInt(this.dataset.index);
        const [movedAgent] = agents.splice(draggedIndex, 1);
        agents.splice(dropIndex, 0, movedAgent);
        renderAgents();
    }
}

// ============================================
// EVENT LISTENERS
// ============================================
function setupEventListeners() {
    startBtn.addEventListener('click', startDebate);
    continueBtn.addEventListener('click', continueDebate);
    stopBtn.addEventListener('click', stopDebate);
    autoBtn.addEventListener('click', toggleAutoMode);
    nextAgentBtn.addEventListener('click', manualNextTurn);
    exportMdBtn.addEventListener('click', exportToMd);
    importMdBtn.addEventListener('click', importFromMd);
    modeSelect.addEventListener('change', changeMode);

    topicInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && !isDebateRunning) {
            e.preventDefault();
            if (hasMessages) {
                continueDebate();
            } else {
                startDebate();
            }
        }
    });
}

// ============================================
// DEBATE CONTROLS (Using Wails bindings)
// ============================================
async function startDebate() {
    const topic = topicInput.value.trim();
    if (!topic) {
        alert('Vui lòng nhập chủ đề thảo luận');
        return;
    }

    try {
        await window.go.main.App.StartDebate(topic);
        isDebateRunning = true;
        hasMessages = false;
        updateControls();
        clearWelcomeMessage();
    } catch (error) {
        console.error('Failed to start debate:', error);
        alert('Lỗi: ' + error);
    }
}

async function continueDebate() {
    const topic = topicInput.value.trim();
    if (!topic) {
        alert('Vui lòng nhập chủ đề mới');
        return;
    }

    try {
        await window.go.main.App.ContinueDebate(topic);
        isDebateRunning = true;
        updateControls();
    } catch (error) {
        console.error('Failed to continue debate:', error);
        alert('Lỗi: ' + error);
    }
}

async function stopDebate() {
    try {
        stopAutoMode();
        pendingTurn = false;

        if (currentStreamingMessage) {
            updateStatus('processing', 'Đang chờ hoàn thành...');
            window.stopAfterCurrentMessage = true;
        } else {
            await window.go.main.App.StopDebate();
            isDebateRunning = false;
            updateControls();
        }
    } catch (error) {
        console.error('Failed to stop debate:', error);
    }
}

async function triggerNextTurn() {
    if (!isDebateRunning) return;

    pendingTurn = false;

    if (isAutoMode) {
        const selectedList = agents.filter(a => selectedAgents.has(a.id));
        if (selectedList.length === 0) {
            console.log('No agents selected for auto mode');
            stopAutoMode();
            return;
        }

        const nextAgent = selectedList[autoAgentIndex % selectedList.length];
        autoAgentIndex++;

        try {
            await window.go.main.App.TriggerAgent(nextAgent.id);
        } catch (error) {
            console.error('Failed to trigger agent turn:', error);
        }
    } else {
        try {
            await window.go.main.App.TriggerNextAgent();
        } catch (error) {
            console.error('Failed to trigger next turn:', error);
        }
    }
}

async function triggerAgentTurn(agentId) {
    if (!isDebateRunning) return;

    try {
        await window.go.main.App.TriggerAgent(agentId);
    } catch (error) {
        console.error('Failed to trigger agent turn:', error);
    }
}

async function manualNextTurn() {
    if (!isDebateRunning) return;

    try {
        await window.go.main.App.TriggerNextAgent();
    } catch (error) {
        console.error('Failed to trigger next turn:', error);
    }
}

async function changeMode() {
    try {
        await window.go.main.App.SetMode(modeSelect.value);
    } catch (error) {
        console.error('Failed to change mode:', error);
    }
}

// ============================================
// AUTO MODE
// ============================================
function toggleAutoMode() {
    if (isAutoMode) {
        stopAutoMode();
    } else {
        startAutoMode();
    }
}

function startAutoMode() {
    isAutoMode = true;
    autoAgentIndex = (hasMessages || currentStreamingMessage || pendingTurn) ? 1 : 0;
    autoBtn.textContent = 'Dừng tự động';
    autoBtn.classList.add('btn-danger');
    autoBtn.classList.remove('btn-secondary');
}

function stopAutoMode() {
    isAutoMode = false;
    autoBtn.textContent = 'Tự động';
    autoBtn.classList.remove('btn-danger');
    autoBtn.classList.add('btn-secondary');
}

// ============================================
// UI HELPERS
// ============================================
function updateControls() {
    startBtn.disabled = isDebateRunning;
    continueBtn.disabled = isDebateRunning || !hasMessages;
    stopBtn.disabled = !isDebateRunning;
    autoBtn.disabled = !isDebateRunning;
    nextAgentBtn.disabled = !isDebateRunning;
    topicInput.disabled = isDebateRunning;
    renderAgents();
}

function updateStatus(status, text) {
    statusIndicator.className = `status-indicator ${status}`;
    statusText.textContent = text;
}

function clearWelcomeMessage() {
    const welcome = messagesContainer.querySelector('.welcome-message');
    if (welcome) {
        welcome.remove();
    }
}

function createMessage(data) {
    const agent = agents.find(a => a.id === data.agent_id) || {
        name: data.agent_name,
        color: data.color || '#666'
    };

    const messageEl = document.createElement('div');
    messageEl.className = 'message';
    messageEl.dataset.messageId = data.message_id;

    messageEl.innerHTML = `
        <div class="avatar" style="background: ${agent.color}">
            ${agent.name.charAt(0)}
        </div>
        <div class="content" style="border-left-color: ${agent.color}">
            <div class="header">
                <span class="name" style="color: ${agent.color}">${agent.name}</span>
                <span class="time">${new Date().toLocaleTimeString('vi-VN')}</span>
            </div>
            <div class="text markdown-body streaming"></div>
        </div>
    `;

    currentStreamingContent = '';
    messagesContainer.appendChild(messageEl);
    scrollToBottom();

    return messageEl;
}

function appendToMessage(messageEl, content) {
    const textEl = messageEl.querySelector('.text');
    currentStreamingContent += content;
    textEl.innerHTML = renderMarkdown(currentStreamingContent);
    scrollToBottom();
}

function finalizeMessage(messageEl) {
    const textEl = messageEl.querySelector('.text');
    textEl.classList.remove('streaming');
    textEl.innerHTML = renderMarkdown(currentStreamingContent);
    // Store raw markdown for export
    messageEl.dataset.rawContent = currentStreamingContent;
    currentStreamingContent = '';
    hasMessages = true;
    updateControls();
}

function renderMarkdown(content) {
    if (!content) return '';

    if (typeof marked !== 'undefined' && marked.parse) {
        try {
            const html = marked.parse(content, {
                breaks: true,
                gfm: true,
            });
            return html;
        } catch (e) {
            console.error('Markdown parse error:', e);
            return escapeHtml(content).replace(/\n/g, '<br>');
        }
    }

    return simpleMarkdown(content);
}

function simpleMarkdown(content) {
    let html = escapeHtml(content);

    html = html.replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>');
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
    html = html.replace(/^###### (.+)$/gm, '<h6>$1</h6>');
    html = html.replace(/^##### (.+)$/gm, '<h5>$1</h5>');
    html = html.replace(/^#### (.+)$/gm, '<h4>$1</h4>');
    html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
    html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
    html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
    html = html.replace(/\*\*\*([^\*]+)\*\*\*/g, '<strong><em>$1</em></strong>');
    html = html.replace(/\*\*([^\*]+)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/(?<!\*)\*([^\*\n]+)\*(?!\*)/g, '<em>$1</em>');
    html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>');
    html = html.replace(/^\* (.+)$/gm, '<li>$1</li>');
    html = html.replace(/^- (.+)$/gm, '<li>$1</li>');
    html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');
    html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
    html = html.replace(/\n\n/g, '</p><p>');
    html = html.replace(/\n/g, '<br>');

    if (!html.startsWith('<')) {
        html = '<p>' + html + '</p>';
    }

    return html;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function addSystemMessage(text) {
    const messageEl = document.createElement('div');
    messageEl.className = 'message system-message';
    messageEl.innerHTML = `
        <div class="avatar" style="background: #888888">
            S
        </div>
        <div class="content" style="border-left-color: #888888">
            <div class="header">
                <span class="name" style="color: #888888">Hệ thống</span>
                <span class="time">${new Date().toLocaleTimeString('vi-VN')}</span>
            </div>
            <div class="text">${text}</div>
        </div>
    `;
    messagesContainer.appendChild(messageEl);
    scrollToBottom();
}

function scrollToBottom() {
    const threshold = 150;
    const isNearBottom = messagesContainer.scrollHeight - messagesContainer.scrollTop - messagesContainer.clientHeight < threshold;

    if (isNearBottom) {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
}

function forceScrollToBottom() {
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
}

function setupScrollButton() {
    if (!scrollToBottomBtn) return;

    const threshold = 150;

    scrollToBottomBtn.removeAttribute('style');
    scrollToBottomBtn.style.display = 'flex';
    scrollToBottomBtn.style.opacity = '0';
    scrollToBottomBtn.style.pointerEvents = 'none';
    scrollToBottomBtn.style.transform = 'translateY(10px)';

    function updateButtonVisibility() {
        const isNearBottom = messagesContainer.scrollHeight - messagesContainer.scrollTop - messagesContainer.clientHeight < threshold;

        if (isNearBottom) {
            scrollToBottomBtn.style.opacity = '0';
            scrollToBottomBtn.style.pointerEvents = 'none';
            scrollToBottomBtn.style.transform = 'translateY(10px)';
        } else {
            scrollToBottomBtn.style.opacity = '1';
            scrollToBottomBtn.style.pointerEvents = 'auto';
            scrollToBottomBtn.style.transform = 'translateY(0)';
        }
    }

    messagesContainer.addEventListener('scroll', updateButtonVisibility);

    scrollToBottomBtn.addEventListener('click', () => {
        messagesContainer.scrollTo({
            top: messagesContainer.scrollHeight,
            behavior: 'smooth'
        });
    });

    updateButtonVisibility();
}

// ============================================
// EXPORT/IMPORT
// ============================================
async function exportToMd() {
    const messages = messagesContainer.querySelectorAll('.message:not(.system-message)');

    if (messages.length === 0) {
        alert('Không có nội dung để xuất. Hãy bắt đầu một cuộc thảo luận trước.');
        return;
    }

    const mdLines = ['# AI Multi-Agent Debate', ''];
    mdLines.push(`**Ngày xuất:** ${new Date().toLocaleString('vi-VN')}`);
    mdLines.push('');
    mdLines.push('---');
    mdLines.push('');

    messages.forEach(message => {
        const nameEl = message.querySelector('.name');
        const timeEl = message.querySelector('.time');

        if (nameEl && timeEl) {
            const agent = nameEl.textContent.trim();
            const time = timeEl.textContent.trim();
            // Use raw markdown content if available, fallback to textContent
            const content = message.dataset.rawContent || message.querySelector('.text')?.textContent?.trim() || '';

            mdLines.push(`## ${agent} *(${time})*`);
            mdLines.push('');
            mdLines.push(content);
            mdLines.push('');
            mdLines.push('---');
            mdLines.push('');
        }
    });

    const mdContent = mdLines.join('\n');
    const defaultFilename = `debate_${new Date().toISOString().slice(0, 10)}_${Date.now()}.md`;

    try {
        const savedPath = await window.go.main.App.SaveToFile(mdContent, defaultFilename);
        if (savedPath) {
            alert(`Đã lưu file thành công:\n${savedPath}`);
        }
    } catch (error) {
        console.error('Export error:', error);
        alert('Lỗi khi xuất file: ' + error);
    }
}

async function importFromMd() {
    try {
        const content = await window.go.main.App.OpenFile();
        if (!content) {
            return; // User cancelled
        }
        parseMdAndDisplay(content);
    } catch (error) {
        console.error('Import error:', error);
        alert('Lỗi khi mở file: ' + error);
    }
}

function parseMdAndDisplay(content) {
    clearWelcomeMessage();
    messagesContainer.innerHTML = '';

    const sections = content.split(/(?=^## )/gm);

    let messageCount = 0;
    sections.forEach(section => {
        const headerMatch = section.match(/^## (.+?) \*\((.+?)\)\*/);
        if (!headerMatch) return;

        const agentName = headerMatch[1].trim();
        const time = headerMatch[2].trim();

        const lines = section.split('\n');
        const contentLines = [];
        let startContent = false;

        for (let i = 1; i < lines.length; i++) {
            const line = lines[i];
            if (line === '---') break;
            if (!startContent && line.trim() === '') continue;
            startContent = true;
            contentLines.push(line);
        }

        const messageContent = contentLines.join('\n').trim();
        if (!messageContent) return;

        const agent = agents.find(a => a.name.toLowerCase() === agentName.toLowerCase()) || {
            name: agentName,
            color: getColorForAgent(agentName)
        };

        messageCount++;
        const messageEl = document.createElement('div');
        messageEl.className = 'message';
        messageEl.dataset.messageId = `imported_${messageCount}`;

        messageEl.innerHTML = `
            <div class="avatar" style="background: ${agent.color}">
                ${agentName.charAt(0)}
            </div>
            <div class="content" style="border-left-color: ${agent.color}">
                <div class="header">
                    <span class="name" style="color: ${agent.color}">${agentName}</span>
                    <span class="time">${time}</span>
                </div>
                <div class="text markdown-body">${renderMarkdown(messageContent)}</div>
            </div>
        `;

        messagesContainer.appendChild(messageEl);
    });

    if (messageCount > 0) {
        hasMessages = true;
        updateControls();
        scrollToBottom();
        alert(`Đã import thành công ${messageCount} tin nhắn!`);
    } else {
        alert('Không tìm thấy tin nhắn hợp lệ trong file MD.');
    }
}

function getColorForAgent(name) {
    const colors = ['#4A90D9', '#9B59B6', '#E74C3C', '#27AE60', '#F39C12', '#1ABC9C'];
    let hash = 0;
    for (let i = 0; i < name.length; i++) {
        hash = name.charCodeAt(i) + ((hash << 5) - hash);
    }
    return colors[Math.abs(hash) % colors.length];
}

// ============================================
// SIDEBAR RESIZE
// ============================================
function setupSidebarResize() {
    const resizeHandle = document.getElementById('resizeHandle');
    const sidebar = document.getElementById('sidebar');

    if (!resizeHandle || !sidebar) return;

    let isResizing = false;
    let startX = 0;
    let startWidth = 0;

    resizeHandle.addEventListener('mousedown', (e) => {
        isResizing = true;
        startX = e.clientX;
        startWidth = sidebar.offsetWidth;
        resizeHandle.classList.add('resizing');
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
        e.preventDefault();
    });

    document.addEventListener('mousemove', (e) => {
        if (!isResizing) return;

        const deltaX = startX - e.clientX;
        const newWidth = Math.min(Math.max(startWidth + deltaX, 200), 500);
        sidebar.style.width = newWidth + 'px';
    });

    document.addEventListener('mouseup', () => {
        if (isResizing) {
            isResizing = false;
            resizeHandle.classList.remove('resizing');
            document.body.style.cursor = '';
            document.body.style.userSelect = '';
        }
    });
}

// ============================================
// AGENT MANAGER
// ============================================
function loadHiddenAgents() {
    const saved = localStorage.getItem('hiddenAgents');
    if (saved) {
        try {
            hiddenAgents = new Set(JSON.parse(saved));
        } catch (e) {
            hiddenAgents = new Set();
        }
    }
}

function saveHiddenAgents() {
    localStorage.setItem('hiddenAgents', JSON.stringify([...hiddenAgents]));
}

async function loadAgentConfigs() {
    try {
        agentConfigs = await window.go.main.App.GetAgentConfigsJS();
        renderAgentManagerList();
    } catch (error) {
        console.error('Failed to load agent configs:', error);
    }
}

function setupAgentManager() {
    const manageBtn = document.getElementById('manageAgentsBtn');
    const addAgentBtn = document.getElementById('addAgentBtn');
    const saveOrderBtn = document.getElementById('saveOrderBtn');
    const agentForm = document.getElementById('agentForm');
    const colorInput = document.getElementById('agentColor');
    const colorText = document.getElementById('agentColorText');

    if (manageBtn) {
        manageBtn.addEventListener('click', openAgentModal);
    }

    if (addAgentBtn) {
        addAgentBtn.addEventListener('click', () => showAgentForm(null));
    }

    if (saveOrderBtn) {
        saveOrderBtn.addEventListener('click', saveAgentOrder);
    }

    if (agentForm) {
        agentForm.addEventListener('submit', handleAgentFormSubmit);
    }

    if (colorInput && colorText) {
        colorInput.addEventListener('input', (e) => {
            colorText.value = e.target.value;
        });
        colorText.addEventListener('input', (e) => {
            if (/^#[0-9A-Fa-f]{6}$/.test(e.target.value)) {
                colorInput.value = e.target.value;
            }
        });
    }

    const modal = document.getElementById('agentModal');
    if (modal) {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                closeAgentModal();
            }
        });
    }
}

function openAgentModal() {
    loadAgentConfigs();
    const modal = document.getElementById('agentModal');
    modal.classList.add('show');
    showAgentList();
}

function closeAgentModal() {
    const modal = document.getElementById('agentModal');
    modal.classList.remove('show');
}

function showAgentList() {
    document.getElementById('agentListView').style.display = 'block';
    document.getElementById('agentEditView').style.display = 'none';
    document.getElementById('agentModalTitle').textContent = 'Quản lý Agents';
    editingAgentId = null;
}

function showAgentForm(agentId) {
    document.getElementById('agentListView').style.display = 'none';
    document.getElementById('agentEditView').style.display = 'block';

    editingAgentId = agentId;
    const form = document.getElementById('agentForm');
    const idInput = document.getElementById('agentId');

    if (agentId) {
        document.getElementById('agentModalTitle').textContent = 'Sửa Agent';
        idInput.disabled = true;

        const agent = agentConfigs.find(a => a.id === agentId);
        if (agent) {
            document.getElementById('agentId').value = agent.id;
            document.getElementById('agentName').value = agent.name;
            document.getElementById('agentRole').value = agent.role || '';
            document.getElementById('agentProvider').value = agent.provider || 'openai';
            document.getElementById('agentModel').value = agent.model || '';
            document.getElementById('agentTemperature').value = agent.temperature || 0.7;
            document.getElementById('agentMaxTokens').value = agent.max_tokens || 0;
            document.getElementById('agentTopP').value = agent.top_p || 1;
            document.getElementById('agentTopK').value = agent.top_k || 0;
            document.getElementById('agentFrequencyPenalty').value = agent.frequency_penalty || 0;
            document.getElementById('agentPresencePenalty').value = agent.presence_penalty || 0;
            document.getElementById('agentColor').value = agent.color || '#4A90D9';
            document.getElementById('agentColorText').value = agent.color || '#4A90D9';
            document.getElementById('agentBaseUrl').value = agent.base_url || '';
            document.getElementById('agentApiKey').value = agent.api_key || '';
            document.getElementById('agentSystemPrompt').value = agent.system_prompt || '';
        }
    } else {
        document.getElementById('agentModalTitle').textContent = 'Thêm Agent Mới';
        idInput.disabled = false;
        form.reset();
        document.getElementById('agentTemperature').value = 0.7;
        document.getElementById('agentMaxTokens').value = 0;
        document.getElementById('agentTopP').value = 1;
        document.getElementById('agentTopK').value = 0;
        document.getElementById('agentFrequencyPenalty').value = 0;
        document.getElementById('agentPresencePenalty').value = 0;
        document.getElementById('agentColor').value = '#4A90D9';
        document.getElementById('agentColorText').value = '#4A90D9';
    }
}

function renderAgentManagerList() {
    const container = document.getElementById('agentManagerList');
    if (!container) return;

    if (agentConfigs.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10"></circle>
                    <line x1="12" y1="8" x2="12" y2="12"></line>
                    <line x1="12" y1="16" x2="12.01" y2="16"></line>
                </svg>
                <p>Chưa có agent nào. Nhấn "Thêm Agent" để tạo mới.</p>
            </div>
        `;
        return;
    }

    container.innerHTML = agentConfigs.map((agent, index) => `
        <div class="agent-manager-item" draggable="true" data-id="${agent.id}" data-index="${index}" style="border-left-color: ${agent.color}">
            <span class="drag-handle">⋮⋮</span>
            <div class="agent-avatar" style="background: ${agent.color}">
                ${agent.name.charAt(0)}
            </div>
            <div class="agent-details">
                <h4>${agent.name}</h4>
                <p>${agent.role || 'Không có vai trò'}</p>
                <span class="agent-meta">${agent.provider} / ${agent.model || 'default'}</span>
            </div>
            <label class="toggle-switch" title="${hiddenAgents.has(agent.id) ? 'Ẩn khỏi panel' : 'Hiển thị trên panel'}">
                <input type="checkbox" ${!hiddenAgents.has(agent.id) ? 'checked' : ''} onchange="toggleAgentVisibility('${agent.id}', this.checked)">
                <span class="toggle-slider"></span>
            </label>
            <div class="agent-actions">
                <button class="btn-icon" onclick="showAgentForm('${agent.id}')" title="Sửa">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                        <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                    </svg>
                </button>
                <button class="btn-icon btn-delete" onclick="deleteAgent('${agent.id}')" title="Xóa">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="3 6 5 6 21 6"></polyline>
                        <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                    </svg>
                </button>
            </div>
        </div>
    `).join('');

    setupManagerDragDrop();
}

function toggleAgentVisibility(agentId, visible) {
    if (visible) {
        hiddenAgents.delete(agentId);
    } else {
        hiddenAgents.add(agentId);
    }
    saveHiddenAgents();
    renderAgents();
}

function setupManagerDragDrop() {
    const container = document.getElementById('agentManagerList');
    if (!container) return;

    const items = container.querySelectorAll('.agent-manager-item');
    let draggedItem = null;

    items.forEach(item => {
        item.addEventListener('dragstart', (e) => {
            draggedItem = item;
            item.classList.add('dragging');
            e.dataTransfer.effectAllowed = 'move';
        });

        item.addEventListener('dragend', () => {
            item.classList.remove('dragging');
            draggedItem = null;
            items.forEach(i => i.classList.remove('drag-over-top'));
        });

        item.addEventListener('dragover', (e) => {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';
        });

        item.addEventListener('dragenter', (e) => {
            e.preventDefault();
            if (item !== draggedItem) {
                item.classList.add('drag-over-top');
            }
        });

        item.addEventListener('dragleave', () => {
            item.classList.remove('drag-over-top');
        });

        item.addEventListener('drop', (e) => {
            e.preventDefault();
            if (item !== draggedItem && draggedItem) {
                const draggedIndex = parseInt(draggedItem.dataset.index);
                const dropIndex = parseInt(item.dataset.index);

                const [moved] = agentConfigs.splice(draggedIndex, 1);
                agentConfigs.splice(dropIndex, 0, moved);

                renderAgentManagerList();
            }
        });
    });
}

async function saveAgentOrder() {
    const agentIds = agentConfigs.map(a => a.id);

    try {
        await window.go.main.App.ReorderAgentsJS(agentIds);
        await loadAgents();
        alert('Đã lưu thứ tự agents!');
    } catch (error) {
        console.error('Failed to save agent order:', error);
        alert('Lỗi: ' + error);
    }
}

async function handleAgentFormSubmit(e) {
    e.preventDefault();

    const formData = {
        id: document.getElementById('agentId').value.trim(),
        name: document.getElementById('agentName').value.trim(),
        role: document.getElementById('agentRole').value.trim(),
        provider: document.getElementById('agentProvider').value,
        model: document.getElementById('agentModel').value.trim(),
        temperature: parseFloat(document.getElementById('agentTemperature').value) || 0.7,
        max_tokens: parseInt(document.getElementById('agentMaxTokens').value) || 0,
        top_p: parseFloat(document.getElementById('agentTopP').value) || 1,
        top_k: parseInt(document.getElementById('agentTopK').value) || 0,
        frequency_penalty: parseFloat(document.getElementById('agentFrequencyPenalty').value) || 0,
        presence_penalty: parseFloat(document.getElementById('agentPresencePenalty').value) || 0,
        color: document.getElementById('agentColor').value,
        base_url: document.getElementById('agentBaseUrl').value.trim(),
        api_key: document.getElementById('agentApiKey').value,
        system_prompt: document.getElementById('agentSystemPrompt').value
    };

    if (!formData.id || !formData.name) {
        alert('ID và Tên là bắt buộc!');
        return;
    }

    if (!/^[a-zA-Z0-9_]+$/.test(formData.id)) {
        alert('ID chỉ được chứa chữ cái, số và dấu gạch dưới!');
        return;
    }

    try {
        if (editingAgentId) {
            await window.go.main.App.UpdateAgentJS(editingAgentId, formData);
        } else {
            await window.go.main.App.AddAgentJS(formData);
        }
        await loadAgentConfigs();
        await loadAgents();
        showAgentList();
    } catch (error) {
        console.error('Failed to save agent:', error);
        alert('Lỗi: ' + error);
    }
}

async function deleteAgent(agentId) {
    const agent = agentConfigs.find(a => a.id === agentId);
    if (!confirm(`Bạn có chắc muốn xóa agent "${agent?.name || agentId}"?`)) {
        return;
    }

    try {
        await window.go.main.App.DeleteAgentJS(agentId);
        await loadAgentConfigs();
        await loadAgents();
    } catch (error) {
        console.error('Failed to delete agent:', error);
        alert('Lỗi: ' + error);
    }
}

// ============================================
// HELP MODAL
// ============================================
function setupHelpModal() {
    const helpBtn = document.getElementById('helpBtn');
    const helpModal = document.getElementById('helpModal');

    if (helpBtn) {
        helpBtn.addEventListener('click', openHelpModal);
    }

    if (helpModal) {
        helpModal.addEventListener('click', (e) => {
            if (e.target === helpModal) {
                closeHelpModal();
            }
        });
    }

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            const helpModal = document.getElementById('helpModal');
            if (helpModal && helpModal.classList.contains('show')) {
                closeHelpModal();
            }
        }
    });
}

function openHelpModal() {
    const modal = document.getElementById('helpModal');
    if (modal) {
        modal.classList.add('show');
    }
}

function closeHelpModal() {
    const modal = document.getElementById('helpModal');
    if (modal) {
        modal.classList.remove('show');
    }
}
