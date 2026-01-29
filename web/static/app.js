// State
let ws = null;
let isConnected = false;
let isDebateRunning = false;
let isAutoMode = false;
let autoInterval = null;
let agents = [];
let agentConfigs = []; // Full agent configs from server
let currentStreamingMessage = null;
let selectedAgents = new Set(); // Agents selected for auto mode
let autoAgentIndex = 0; // Current index for round-robin in auto mode
let hiddenAgents = new Set(); // Agents hidden from the panel
let editingAgentId = null; // Currently editing agent ID (null for new)
let pendingTurn = false; // Flag to prevent double triggering
let currentStreamingContent = ''; // Raw content being streamed for markdown

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
let hasMessages = false;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    connectWebSocket();
    loadAgents();
    loadHiddenAgents();
    setupEventListeners();
    setupSidebarResize();
    setupAgentManager();
    setupHelpModal();
    setupScrollButton();

    // Check if marked.js is loaded
    if (typeof marked === 'undefined') {
        console.warn('marked.js not loaded, using fallback markdown renderer');
    } else {
        console.log('marked.js loaded successfully');
    }
});

// WebSocket Connection
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        isConnected = true;
        updateStatus('online', 'Đã kết nối');
        console.log('WebSocket connected');
    };

    ws.onclose = () => {
        isConnected = false;
        updateStatus('offline', 'Mất kết nối');
        console.log('WebSocket disconnected');
        // Reconnect after 3 seconds
        setTimeout(connectWebSocket, 3000);
    };

    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        updateStatus('offline', 'Lỗi kết nối');
    };

    ws.onmessage = (event) => {
        // Handle potentially batched messages (multiple JSON objects)
        const rawData = event.data;
        const messages = rawData.split('\n').filter(line => line.trim());

        for (const msgStr of messages) {
            try {
                const data = JSON.parse(msgStr);
                handleWebSocketMessage(data);
            } catch (e) {
                console.error('Failed to parse message:', msgStr, e);
            }
        }
    };
}

// Handle WebSocket Messages
function handleWebSocketMessage(data) {
    console.log('WS Message:', data);

    switch (data.type) {
        case 'debate_started':
            isDebateRunning = true;
            updateControls();
            clearWelcomeMessage();
            break;

        case 'debate_stopped':
            isDebateRunning = false;
            stopAutoMode();
            updateControls();
            break;

        case 'debate_reset':
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
            break;

        case 'mode_changed':
            modeSelect.value = data.mode;
            break;

        case 'topic_changed':
            isDebateRunning = true;
            updateControls();
            // Add system message about topic change
            addSystemMessage(`Chuyển sang chủ đề mới: ${data.topic}`);
            break;

        case 'start':
            // Start of a new message
            currentStreamingMessage = createMessage(data);
            updateStatus('processing', `${data.agent_name} đang suy nghĩ...`);
            break;

        case 'chunk':
            // Streaming content
            if (currentStreamingMessage) {
                appendToMessage(currentStreamingMessage, data.content);
            }
            break;

        case 'end':
            // End of message
            if (currentStreamingMessage) {
                finalizeMessage(currentStreamingMessage);
                currentStreamingMessage = null;
            }
            updateStatus('online', 'Sẵn sàng');

            // Check if we should stop after this message
            if (window.stopAfterCurrentMessage) {
                window.stopAfterCurrentMessage = false;
                // Now actually stop the debate
                fetch('/api/debate/stop', { method: 'POST' }).then(() => {
                    isDebateRunning = false;
                    updateControls();
                });
                break;
            }

            // If auto mode, trigger next turn (only if no pending turn already scheduled)
            if (isAutoMode && isDebateRunning && !pendingTurn) {
                pendingTurn = true;
                setTimeout(triggerNextTurn, 2000);
            }
            break;

        case 'error':
            console.error('Debate error:', data.error);
            // Parse and display user-friendly error message
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
            break;

        case 'agents_updated':
            // Reload agents when they are updated from another client or the modal
            loadAgents();
            loadAgentConfigs();
            break;
    }
}

// Load Agents
async function loadAgents() {
    try {
        const response = await fetch('/api/agents');
        agents = await response.json();
        renderAgents();
    } catch (error) {
        console.error('Failed to load agents:', error);
    }
}

// Render Agents as clickable buttons with checkboxes
function renderAgents() {
    // Filter out hidden agents
    const visibleAgents = agents.filter(a => !hiddenAgents.has(a.id));

    // Initialize selectedAgents with all visible agents if empty
    if (selectedAgents.size === 0 && visibleAgents.length > 0) {
        visibleAgents.forEach(agent => selectedAgents.add(agent.id));
    }

    // Remove hidden agents from selectedAgents
    hiddenAgents.forEach(id => selectedAgents.delete(id));

    // Get selected agents in order for numbering
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

    // Add click handlers to agent buttons
    document.querySelectorAll('.agent-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const agentId = btn.dataset.agentId;
            triggerAgentTurn(agentId);
        });
    });

    // Add change handlers to checkboxes
    document.querySelectorAll('.agent-checkbox').forEach(checkbox => {
        checkbox.addEventListener('change', (e) => {
            const agentId = e.target.dataset.agentId;
            if (e.target.checked) {
                selectedAgents.add(agentId);
            } else {
                selectedAgents.delete(agentId);
            }
            // Re-render to update order numbers
            renderAgents();
        });
    });

    // Add drag-drop handlers
    setupDragDrop();
}

// Drag and Drop functionality
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

        // Reorder agents array
        const [movedAgent] = agents.splice(draggedIndex, 1);
        agents.splice(dropIndex, 0, movedAgent);

        // Re-render with new order
        renderAgents();
    }
}

// Event Listeners
function setupEventListeners() {
    startBtn.addEventListener('click', startDebate);
    continueBtn.addEventListener('click', continueDebate);
    stopBtn.addEventListener('click', stopDebate);
    autoBtn.addEventListener('click', toggleAutoMode);
    nextAgentBtn.addEventListener('click', manualNextTurn);
    exportMdBtn.addEventListener('click', exportToMd);
    importMdBtn.addEventListener('click', () => importMdInput.click());
    importMdInput.addEventListener('change', importFromMd);
    modeSelect.addEventListener('change', changeMode);

    topicInput.addEventListener('keydown', (e) => {
        // Ctrl+Enter or Cmd+Enter to start/continue debate
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

// API Calls
async function startDebate() {
    const topic = topicInput.value.trim();
    if (!topic) {
        alert('Vui lòng nhập chủ đề thảo luận');
        return;
    }

    try {
        const response = await fetch('/api/debate/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ topic })
        });

        if (response.ok) {
            isDebateRunning = true;
            hasMessages = false;
            updateControls();
            clearWelcomeMessage();
            // Auto trigger first turn - set pending flag to prevent double triggering
            pendingTurn = true;
            setTimeout(triggerNextTurn, 500);
        }
    } catch (error) {
        console.error('Failed to start debate:', error);
    }
}

async function continueDebate() {
    const topic = topicInput.value.trim();
    if (!topic) {
        alert('Vui lòng nhập chủ đề mới');
        return;
    }

    try {
        const response = await fetch('/api/debate/continue', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ topic })
        });

        if (response.ok) {
            isDebateRunning = true;
            updateControls();
            // Auto trigger next turn - set pending flag to prevent double triggering
            pendingTurn = true;
            setTimeout(triggerNextTurn, 500);
        }
    } catch (error) {
        console.error('Failed to continue debate:', error);
    }
}

async function stopDebate() {
    try {
        // Stop auto mode immediately so no new turns are triggered
        stopAutoMode();
        pendingTurn = false;

        // If there's a message being streamed, wait for it to finish
        // The debate will stop after the current message ends
        if (currentStreamingMessage) {
            updateStatus('processing', 'Đang chờ hoàn thành...');
            // Set flag to stop after current message ends
            window.stopAfterCurrentMessage = true;
        } else {
            // No active message, stop immediately
            await fetch('/api/debate/stop', { method: 'POST' });
            isDebateRunning = false;
            updateControls();
        }
    } catch (error) {
        console.error('Failed to stop debate:', error);
    }
}

async function triggerNextTurn() {
    if (!isDebateRunning) return;

    // Clear pending flag since we're now executing
    pendingTurn = false;

    // If in auto mode, use only selected agents
    if (isAutoMode) {
        const selectedList = agents.filter(a => selectedAgents.has(a.id));
        if (selectedList.length === 0) {
            console.log('No agents selected for auto mode');
            stopAutoMode();
            return;
        }

        // Get next agent in round-robin
        const nextAgent = selectedList[autoAgentIndex % selectedList.length];
        autoAgentIndex++;

        try {
            await fetch(`/api/debate/agent/${nextAgent.id}`, { method: 'POST' });
        } catch (error) {
            console.error('Failed to trigger agent turn:', error);
        }
    } else {
        try {
            await fetch('/api/debate/next', { method: 'POST' });
        } catch (error) {
            console.error('Failed to trigger next turn:', error);
        }
    }
}

async function triggerAgentTurn(agentId) {
    if (!isDebateRunning) return;

    try {
        await fetch(`/api/debate/agent/${agentId}`, { method: 'POST' });
    } catch (error) {
        console.error('Failed to trigger agent turn:', error);
    }
}

// Manual trigger for next agent turn (non-auto mode)
async function manualNextTurn() {
    if (!isDebateRunning) return;

    try {
        await fetch('/api/debate/next', { method: 'POST' });
    } catch (error) {
        console.error('Failed to trigger next turn:', error);
    }
}

async function changeMode() {
    try {
        await fetch('/api/debate/mode', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ mode: modeSelect.value })
        });
    } catch (error) {
        console.error('Failed to change mode:', error);
    }
}

// Auto Mode
function toggleAutoMode() {
    if (isAutoMode) {
        stopAutoMode();
    } else {
        startAutoMode();
    }
}

function startAutoMode() {
    isAutoMode = true;
    // If there's already a message, a streaming message, or a pending turn,
    // start from index 1 to skip the agent that's already been triggered.
    // This prevents double-triggering the first agent when user clicks auto
    // right after starting debate.
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

// UI Helpers
function updateControls() {
    startBtn.disabled = isDebateRunning;
    continueBtn.disabled = isDebateRunning || !hasMessages;
    stopBtn.disabled = !isDebateRunning;
    autoBtn.disabled = !isDebateRunning;
    nextAgentBtn.disabled = !isDebateRunning;
    topicInput.disabled = isDebateRunning;

    // Re-render agents to update their disabled state
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
        color: '#666'
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

    // Reset streaming content for new message
    currentStreamingContent = '';

    messagesContainer.appendChild(messageEl);
    scrollToBottom();

    return messageEl;
}

function appendToMessage(messageEl, content) {
    const textEl = messageEl.querySelector('.text');
    currentStreamingContent += content;
    // Render markdown during streaming
    textEl.innerHTML = renderMarkdown(currentStreamingContent);
    scrollToBottom();
}

function finalizeMessage(messageEl) {
    const textEl = messageEl.querySelector('.text');
    textEl.classList.remove('streaming');
    // Final render of markdown
    textEl.innerHTML = renderMarkdown(currentStreamingContent);
    currentStreamingContent = '';
    hasMessages = true;
    updateControls();
}

// Render markdown content safely
function renderMarkdown(content) {
    if (!content) return '';

    // Check if marked library is loaded
    if (typeof marked !== 'undefined' && marked.parse) {
        try {
            // Use marked.parse with options for safety
            const html = marked.parse(content, {
                breaks: true,        // Convert \n to <br>
                gfm: true,           // GitHub Flavored Markdown
            });
            return html;
        } catch (e) {
            console.error('Markdown parse error:', e);
            return escapeHtml(content).replace(/\n/g, '<br>');
        }
    }

    // Fallback: simple markdown rendering
    return simpleMarkdown(content);
}

// Simple markdown fallback when marked.js is not loaded
function simpleMarkdown(content) {
    let html = escapeHtml(content);

    // Code blocks first (before other processing)
    html = html.replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>');

    // Inline code (before bold/italic to avoid conflicts)
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

    // Headers (must be at start of line)
    html = html.replace(/^###### (.+)$/gm, '<h6>$1</h6>');
    html = html.replace(/^##### (.+)$/gm, '<h5>$1</h5>');
    html = html.replace(/^#### (.+)$/gm, '<h4>$1</h4>');
    html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
    html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
    html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');

    // Bold and italic (handle multi-line with [\s\S])
    html = html.replace(/\*\*\*([^\*]+)\*\*\*/g, '<strong><em>$1</em></strong>');
    html = html.replace(/\*\*([^\*]+)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/(?<!\*)\*([^\*\n]+)\*(?!\*)/g, '<em>$1</em>');

    // Also support __bold__ and _italic_
    html = html.replace(/___([^_]+)___/g, '<strong><em>$1</em></strong>');
    html = html.replace(/__([^_]+)__/g, '<strong>$1</strong>');
    html = html.replace(/(?<!_)_([^_\n]+)_(?!_)/g, '<em>$1</em>');

    // Blockquotes
    html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>');

    // Unordered lists
    html = html.replace(/^\* (.+)$/gm, '<li>$1</li>');
    html = html.replace(/^- (.+)$/gm, '<li>$1</li>');

    // Ordered lists
    html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');

    // Horizontal rule
    html = html.replace(/^---$/gm, '<hr>');
    html = html.replace(/^\*\*\*$/gm, '<hr>');

    // Links
    html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');

    // Line breaks (convert double newlines to paragraphs, single to <br>)
    html = html.replace(/\n\n/g, '</p><p>');
    html = html.replace(/\n/g, '<br>');

    // Wrap in paragraph if not already wrapped
    if (!html.startsWith('<')) {
        html = '<p>' + html + '</p>';
    }

    // Clean up consecutive <br> after block elements
    html = html.replace(/(<\/h[1-6]>)<br>/g, '$1');
    html = html.replace(/(<\/li>)<br>/g, '$1');
    html = html.replace(/(<\/blockquote>)<br>/g, '$1');
    html = html.replace(/(<\/pre>)<br>/g, '$1');
    html = html.replace(/(<hr>)<br>/g, '$1');
    html = html.replace(/(<\/p>)<br>/g, '$1');
    html = html.replace(/<p><\/p>/g, '');

    return html;
}

// Escape HTML to prevent XSS
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
    // Only auto-scroll if user is already near the bottom (within 150px)
    // This allows users to read previous messages without being interrupted
    const threshold = 150;
    const isNearBottom = messagesContainer.scrollHeight - messagesContainer.scrollTop - messagesContainer.clientHeight < threshold;

    if (isNearBottom) {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
}

// Force scroll to bottom (used when user explicitly wants to go to bottom)
function forceScrollToBottom() {
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
}

// Setup scroll to bottom button
function setupScrollButton() {
    if (!scrollToBottomBtn) return;

    const threshold = 150;

    // CRITICAL: Completely remove any cached inline style attribute first
    // This clears any browser-cached inline 'display:none' from old HTML versions
    scrollToBottomBtn.removeAttribute('style');

    // Now set fresh inline styles for initial hidden state
    scrollToBottomBtn.style.display = 'flex';
    scrollToBottomBtn.style.opacity = '0';
    scrollToBottomBtn.style.pointerEvents = 'none';
    scrollToBottomBtn.style.transform = 'translateY(10px)';

    console.log('Scroll button initialized:', scrollToBottomBtn.style.cssText);

    // Function to show/hide button
    function updateButtonVisibility() {
        const isNearBottom = messagesContainer.scrollHeight - messagesContainer.scrollTop - messagesContainer.clientHeight < threshold;

        if (isNearBottom) {
            // Hide button
            scrollToBottomBtn.style.opacity = '0';
            scrollToBottomBtn.style.pointerEvents = 'none';
            scrollToBottomBtn.style.transform = 'translateY(10px)';
        } else {
            // Show button
            scrollToBottomBtn.style.opacity = '1';
            scrollToBottomBtn.style.pointerEvents = 'auto';
            scrollToBottomBtn.style.transform = 'translateY(0)';
        }
    }

    // Show/hide button based on scroll position
    messagesContainer.addEventListener('scroll', updateButtonVisibility);

    // Click to scroll to bottom
    scrollToBottomBtn.addEventListener('click', () => {
        messagesContainer.scrollTo({
            top: messagesContainer.scrollHeight,
            behavior: 'smooth'
        });
    });

    // Initial check
    updateButtonVisibility();
}

// Export to Markdown
function exportToMd() {
    const messages = messagesContainer.querySelectorAll('.message:not(.system-message)');

    if (messages.length === 0) {
        alert('Không có nội dung để xuất. Hãy bắt đầu một cuộc thảo luận trước.');
        return;
    }

    // Build Markdown content
    const mdLines = ['# AI Multi-Agent Debate', ''];
    mdLines.push(`**Ngày xuất:** ${new Date().toLocaleString('vi-VN')}`);
    mdLines.push('');
    mdLines.push('---');
    mdLines.push('');

    messages.forEach(message => {
        const nameEl = message.querySelector('.name');
        const timeEl = message.querySelector('.time');
        const textEl = message.querySelector('.text');

        if (nameEl && timeEl && textEl) {
            const agent = nameEl.textContent.trim();
            const time = timeEl.textContent.trim();
            const content = textEl.textContent.trim();

            mdLines.push(`## ${agent} *(${time})*`);
            mdLines.push('');
            mdLines.push(content);
            mdLines.push('');
            mdLines.push('---');
            mdLines.push('');
        }
    });

    // Create and download MD file with UTF-8 BOM for better compatibility
    const mdContent = mdLines.join('\n');
    const BOM = '\uFEFF'; // UTF-8 BOM for better text editor compatibility
    const blob = new Blob([BOM + mdContent], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);

    const link = document.createElement('a');
    link.setAttribute('href', url);
    link.setAttribute('download', `debate_${new Date().toISOString().slice(0, 10)}_${Date.now()}.md`);
    link.style.display = 'none';

    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    URL.revokeObjectURL(url);
}

// Import from Markdown
function importFromMd(event) {
    const file = event.target.files[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = function (e) {
        const content = e.target.result;
        parseMdAndDisplay(content);
    };
    reader.readAsText(file);

    // Reset input để có thể chọn lại cùng file
    event.target.value = '';
}

function parseMdAndDisplay(content) {
    // Clear welcome message
    clearWelcomeMessage();

    // Clear existing messages
    messagesContainer.innerHTML = '';

    // Parse MD content - tìm các section có format: ## Agent *(time)*
    const sections = content.split(/(?=^## )/gm);

    let messageCount = 0;
    sections.forEach(section => {
        // Match: ## AgentName *(time)*
        const headerMatch = section.match(/^## (.+?) \*\((.+?)\)\*/);
        if (!headerMatch) return;

        const agentName = headerMatch[1].trim();
        const time = headerMatch[2].trim();

        // Get content after header (skip header line and empty lines)
        const lines = section.split('\n');
        const contentLines = [];
        let startContent = false;

        for (let i = 1; i < lines.length; i++) {
            const line = lines[i];
            if (line === '---') break; // Stop at separator
            if (!startContent && line.trim() === '') continue; // Skip leading empty lines
            startContent = true;
            contentLines.push(line);
        }

        const messageContent = contentLines.join('\n').trim();
        if (!messageContent) return;

        // Find agent color
        const agent = agents.find(a => a.name.toLowerCase() === agentName.toLowerCase()) || {
            name: agentName,
            color: getColorForAgent(agentName)
        };

        // Create message element
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

// Helper function to get color for unknown agents
function getColorForAgent(name) {
    const colors = ['#4A90D9', '#9B59B6', '#E74C3C', '#27AE60', '#F39C12', '#1ABC9C'];
    let hash = 0;
    for (let i = 0; i < name.length; i++) {
        hash = name.charCodeAt(i) + ((hash << 5) - hash);
    }
    return colors[Math.abs(hash) % colors.length];
}

// Sidebar Resize Functionality
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
// AGENT MANAGER FUNCTIONS
// ============================================

// Load hidden agents from localStorage
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

// Save hidden agents to localStorage
function saveHiddenAgents() {
    localStorage.setItem('hiddenAgents', JSON.stringify([...hiddenAgents]));
}

// Load full agent configs from server
async function loadAgentConfigs() {
    try {
        const response = await fetch('/api/agents/configs');
        agentConfigs = await response.json();
        renderAgentManagerList();
    } catch (error) {
        console.error('Failed to load agent configs:', error);
    }
}

// Setup Agent Manager
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

    // Sync color picker with text input
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

    // Close modal when clicking outside
    const modal = document.getElementById('agentModal');
    if (modal) {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                closeAgentModal();
            }
        });
    }
}

// Open Agent Manager Modal
function openAgentModal() {
    loadAgentConfigs();
    const modal = document.getElementById('agentModal');
    modal.classList.add('show');
    showAgentList();
}

// Close Agent Manager Modal
function closeAgentModal() {
    const modal = document.getElementById('agentModal');
    modal.classList.remove('show');
}

// Show Agent List View
function showAgentList() {
    document.getElementById('agentListView').style.display = 'block';
    document.getElementById('agentEditView').style.display = 'none';
    document.getElementById('agentModalTitle').textContent = 'Quản lý Agents';
    editingAgentId = null;
}

// Show Agent Form (for add/edit)
function showAgentForm(agentId) {
    document.getElementById('agentListView').style.display = 'none';
    document.getElementById('agentEditView').style.display = 'block';

    editingAgentId = agentId;
    const form = document.getElementById('agentForm');
    const idInput = document.getElementById('agentId');

    if (agentId) {
        // Edit mode
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
        // Add mode
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

// Render Agent Manager List
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

    // Setup drag-drop for manager list
    setupManagerDragDrop();
}

// Toggle Agent Visibility
function toggleAgentVisibility(agentId, visible) {
    if (visible) {
        hiddenAgents.delete(agentId);
    } else {
        hiddenAgents.add(agentId);
    }
    saveHiddenAgents();
    renderAgents(); // Re-render the sidebar
}

// Setup Drag-Drop for Manager List
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

                // Reorder agentConfigs
                const [moved] = agentConfigs.splice(draggedIndex, 1);
                agentConfigs.splice(dropIndex, 0, moved);

                // Re-render
                renderAgentManagerList();
            }
        });
    });
}

// Save Agent Order
async function saveAgentOrder() {
    const agentIds = agentConfigs.map(a => a.id);

    try {
        const response = await fetch('/api/agents/reorder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ agent_ids: agentIds })
        });

        if (response.ok) {
            // Reload agents to reflect new order
            await loadAgents();
            alert('Đã lưu thứ tự agents!');
        } else {
            const data = await response.json();
            alert('Lỗi: ' + (data.error || 'Không thể lưu thứ tự'));
        }
    } catch (error) {
        console.error('Failed to save agent order:', error);
        alert('Lỗi kết nối server');
    }
}

// Handle Agent Form Submit
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

    // Validate ID format (no spaces, alphanumeric and underscore only)
    if (!/^[a-zA-Z0-9_]+$/.test(formData.id)) {
        alert('ID chỉ được chứa chữ cái, số và dấu gạch dưới!');
        return;
    }

    try {
        let response;
        if (editingAgentId) {
            // Update existing agent
            response = await fetch(`/api/agents/${editingAgentId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData)
            });
        } else {
            // Add new agent
            response = await fetch('/api/agents', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData)
            });
        }

        if (response.ok) {
            await loadAgentConfigs();
            await loadAgents();
            showAgentList();
        } else {
            const data = await response.json();
            alert('Lỗi: ' + (data.error || 'Không thể lưu agent'));
        }
    } catch (error) {
        console.error('Failed to save agent:', error);
        alert('Lỗi kết nối server');
    }
}

// Delete Agent
async function deleteAgent(agentId) {
    const agent = agentConfigs.find(a => a.id === agentId);
    if (!confirm(`Bạn có chắc muốn xóa agent "${agent?.name || agentId}"?`)) {
        return;
    }

    try {
        const response = await fetch(`/api/agents/${agentId}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            await loadAgentConfigs();
            await loadAgents();
        } else {
            const data = await response.json();
            alert('Lỗi: ' + (data.error || 'Không thể xóa agent'));
        }
    } catch (error) {
        console.error('Failed to delete agent:', error);
        alert('Lỗi kết nối server');
    }
}

// ============================================
// HELP MODAL FUNCTIONS
// ============================================

// Setup Help Modal
function setupHelpModal() {
    const helpBtn = document.getElementById('helpBtn');
    const helpModal = document.getElementById('helpModal');

    if (helpBtn) {
        helpBtn.addEventListener('click', openHelpModal);
    }

    // Close modal when clicking outside
    if (helpModal) {
        helpModal.addEventListener('click', (e) => {
            if (e.target === helpModal) {
                closeHelpModal();
            }
        });
    }

    // Close on ESC key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            const helpModal = document.getElementById('helpModal');
            if (helpModal && helpModal.classList.contains('show')) {
                closeHelpModal();
            }
        }
    });
}

// Open Help Modal
function openHelpModal() {
    const modal = document.getElementById('helpModal');
    if (modal) {
        modal.classList.add('show');
    }
}

// Close Help Modal
function closeHelpModal() {
    const modal = document.getElementById('helpModal');
    if (modal) {
        modal.classList.remove('show');
    }
}
