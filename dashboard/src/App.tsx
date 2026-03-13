import { useState, useEffect } from 'react';
import { Sidebar } from './components/Sidebar';
import { AgentsTab } from './components/AgentsTab';
import { HealthTab } from './components/HealthTab';
import { AppsTab } from './components/AppsTab';
import { SettingsTab } from './components/SettingsTab';
import { ContainersTab } from './components/ContainersTab';
import { CalendarTab } from './components/CalendarTab';
import { AllowlistTab } from './components/AllowlistTab';
import { CreateAgentModal } from './components/modals/CreateAgentModal';
import { TestModal } from './components/modals/TestModal';
import { SkillsModal } from './components/modals/SkillsModal';
import { LogsModal } from './components/modals/LogsModal';
import { ConfigureModal } from './components/modals/ConfigureModal';
import { WorkspaceModal } from './components/modals/WorkspaceModal';
import { ToastContainer } from './components/ToastContainer';
import { Agent, ToastMessage, ContainerItem } from './types';

const API_BASE = '';

export default function App() {
  const [currentTab, setCurrentTab] = useState('agents');
  const [activeModal, setActiveModal] = useState<string | null>(null);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const [selectedContainer, setSelectedContainer] = useState<ContainerItem | null>(null);
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [showLogin, setShowLogin] = useState(true);

  const triggerToast = (message: string, type: 'success' | 'error' | 'info' = 'success') => {
    const id = Date.now();
    setToasts(prev => [...prev, { id, message, type }]);
    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== id));
    }, 3000);
  };

  const openModal = (modalName: string, data?: any) => {
    if (modalName === 'workspace' && data) {
      setSelectedContainer(data);
    } else if (data) {
      setSelectedAgent(data);
    }
    setActiveModal(modalName);
  };

  const closeModal = () => {
    setActiveModal(null);
    setSelectedAgent(null);
    setSelectedContainer(null);
  };

  const fetchAgents = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/agents`);
      const data = await res.json();
      setAgents(data);
    } catch (err) {
      console.error('Failed to fetch agents:', err);
    }
  };

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const res = await fetch(`${API_BASE}/api/auth/check`);
        const data = await res.json();
        if (data.authenticated) {
          setIsAuthenticated(true);
          setShowLogin(false);
          fetchAgents();
        }
      } catch (err) {
        console.error('Auth check failed:', err);
      }
    };
    checkAuth();
  }, []);

  const handleLogout = async () => {
    try {
      await fetch(`${API_BASE}/api/auth/logout`, { method: 'POST' });
    } catch (err) {
      console.error('Logout failed:', err);
    }
    setIsAuthenticated(false);
    setShowLogin(true);
    setAgents([]);
  };

  const handleLogin = async (username: string, password: string) => {
    try {
      const res = await fetch(`${API_BASE}/api/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });
      const data = await res.json();
      if (data.success) {
        setIsAuthenticated(true);
        setShowLogin(false);
        fetchAgents();
        if (data.mustChangePassword) {
          triggerToast('Please change your password', 'info');
        }
      } else {
        triggerToast(data.error || 'Login failed', 'error');
      }
    } catch (err) {
      triggerToast('Login failed', 'error');
    }
  };

  if (showLogin) {
    return (
      <LoginScreen 
        onLogin={handleLogin} 
      />
    );
  }

  return (
    <div className="h-screen w-full overflow-hidden flex bg-black text-white selection:bg-white selection:text-black font-sans">
      <Sidebar currentTab={currentTab} setCurrentTab={setCurrentTab} onLogout={handleLogout} />
      
      <main className="flex-1 h-full py-6 pr-6 pl-0">
        <div className="w-full h-full bg-zinc-950 rounded-[3rem] border border-zinc-800/50 p-12 overflow-y-auto relative flex flex-col shadow-2xl">
          <header className="flex justify-between items-end mb-12 shrink-0">
            <div>
              <h1 className="text-5xl font-black tracking-tighter lowercase">
                {currentTab === 'agents' ? 'your agents' : 
                 currentTab === 'health' ? 'system health' : 
                 currentTab === 'apps' ? 'published apps' : 
                 currentTab === 'containers' ? 'containers' : 
                 currentTab === 'calendar' ? 'calendar' :
                 currentTab === 'allowlist' ? 'allowed users' :
                 currentTab === 'settings' ? 'settings' : ''}
              </h1>
              <div className="flex items-center gap-3 mt-4 text-sm font-medium text-zinc-500">
                <div className="w-2.5 h-2.5 rounded-full bg-white animate-pulse"></div>
                <span>live system connection active</span>
              </div>
            </div>
            
            {currentTab === 'agents' && (
              <button 
                onClick={() => openModal('createAgent')}
                className="bg-white text-black px-8 py-4 rounded-full font-bold text-sm hover:scale-105 transition-all flex items-center gap-2"
              >
                <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14"/><path d="M12 5v14"/></svg>
                deploy new agent
              </button>
            )}
          </header>

          {currentTab === 'agents' && <AgentsTab agents={agents} openModal={openModal} triggerToast={triggerToast} fetchAgents={fetchAgents} />}
          {currentTab === 'health' && <HealthTab />}
          {currentTab === 'apps' && <AppsTab triggerToast={triggerToast} agents={agents} />}
          {currentTab === 'containers' && <ContainersTab openModal={openModal} triggerToast={triggerToast} />}
          {currentTab === 'calendar' && <CalendarTab triggerToast={triggerToast} agents={agents} />}
          {currentTab === 'allowlist' && <AllowlistTab triggerToast={triggerToast} />}
          {currentTab === 'settings' && <SettingsTab triggerToast={triggerToast} />}
        </div>
      </main>

      {activeModal === 'createAgent' && <CreateAgentModal onClose={closeModal} triggerToast={triggerToast} fetchAgents={fetchAgents} />}
      {activeModal === 'testConsole' && selectedAgent && <TestModal agent={selectedAgent} onClose={closeModal} />}
      {activeModal === 'skills' && selectedAgent && <SkillsModal agent={selectedAgent} onClose={closeModal} triggerToast={triggerToast} />}
      {activeModal === 'logs' && selectedAgent && <LogsModal agent={selectedAgent} onClose={closeModal} />}
      {activeModal === 'configure' && selectedAgent && <ConfigureModal agent={selectedAgent} onClose={closeModal} triggerToast={triggerToast} />}
      {activeModal === 'workspace' && selectedContainer && <WorkspaceModal container={selectedContainer} onClose={closeModal} triggerToast={triggerToast} />}
      
      <ToastContainer toasts={toasts} />
    </div>
  );
}

function LoginScreen({ onLogin }: { onLogin: (u: string, p: string) => void }) {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('hermit123');

  return (
    <div className="h-screen w-full flex items-center justify-center bg-black">
      <div className="bg-zinc-950 border border-zinc-800 rounded-[3rem] p-12 w-full max-w-md shadow-2xl">
        <div className="text-center mb-8">
          <svg viewBox="0 0 100 100" className="w-20 h-20 mx-auto mb-4 drop-shadow-[0_0_20px_rgba(255,255,255,0.3)]">
            <line x1="25" y1="45" x2="5" y2="40" stroke="white" strokeWidth="4" strokeLinecap="round" />
            <line x1="23" y1="55" x2="5" y2="55" stroke="white" strokeWidth="4" strokeLinecap="round" />
            <line x1="28" y1="65" x2="10" y2="75" stroke="white" strokeWidth="4" strokeLinecap="round" />
            <line x1="75" y1="45" x2="95" y2="40" stroke="white" strokeWidth="4" strokeLinecap="round" />
            <line x1="77" y1="55" x2="95" y2="55" stroke="white" strokeWidth="4" strokeLinecap="round" />
            <line x1="72" y1="65" x2="90" y2="75" stroke="white" strokeWidth="4" strokeLinecap="round" />
            <circle cx="50" cy="50" r="30" fill="white" />
            <circle cx="42" cy="45" r="5" fill="black" />
            <circle cx="60" cy="45" r="5" fill="black" />
          </svg>
          <h1 className="text-4xl font-black tracking-tighter">HERMIT</h1>
          <p className="text-zinc-500 mt-2">Agent Orchestration System</p>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2 ml-4">Username</label>
            <input 
              type="text" 
              value={username}
              onChange={e => setUsername(e.target.value)}
              className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500"
            />
          </div>
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2 ml-4">Password</label>
            <input 
              type="password" 
              value={password}
              onChange={e => setPassword(e.target.value)}
              className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500"
            />
          </div>
          <button 
            onClick={() => onLogin(username, password)}
            className="w-full bg-white text-black py-4 rounded-full font-bold mt-4 hover:bg-zinc-200 transition-colors"
          >
            Login
          </button>
        </div>
      </div>
    </div>
  );
}
