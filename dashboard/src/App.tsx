// HermitShell Dashboard - React Frontend
// Documentation:
// - frontend-deployment.md: Build process, Vite configuration
// - frontend-backend-communication.md: API calls, authentication flow
// - authentication.md: Login/logout flow with cookies
import { useState, useEffect } from 'react';
import { Sidebar } from './components/Sidebar';
import { AgentsTab } from './components/AgentsTab';
import { HealthTab } from './components/HealthTab';
import { AppsTab } from './components/AppsTab';
import { SettingsTab } from './components/SettingsTab';
import { ContainersTab } from './components/ContainersTab';
import { CalendarTab } from './components/CalendarTab';
import { AllowlistTab } from './components/AllowlistTab';
import { LogsTab } from './components/LogsTab';
import { CreateAgentModal } from './components/modals/CreateAgentModal';
import { TestModal } from './components/modals/TestModal';
import { SkillsModal } from './components/modals/SkillsModal';
import { LogsModal } from './components/modals/LogsModal';
import { ConfigureModal } from './components/modals/ConfigureModal';
import { WorkspaceModal } from './components/modals/WorkspaceModal';
import { ToastContainer } from './components/ToastContainer';
import { DocsTab } from './components/DocsTab';
import { Agent, ToastMessage, ContainerItem } from './types';
import { Clock } from 'lucide-react';

const API_BASE = '';

// SystemClock displays the current time with user's offset applied.
// Docs: See docs/time-management.md for how time is fetched and displayed.
// Purpose: Shows real-time clock in the header using time offset from settings.
function SystemClock() {
  const [time, setTime] = useState({ time: '', time12: '', date: '', timezone: '', timeOffset: '' });

  useEffect(() => {
    const fetchTime = async () => {
      try {
        const res = await fetch(`${API_BASE}/api/time`);
        const data = await res.json();
        setTime(data);
      } catch (err) {
        console.error('Failed to fetch time:', err);
      }
    };

    fetchTime();
    const interval = setInterval(fetchTime, 1000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="flex items-center gap-2 px-4 py-2 bg-zinc-900/50 rounded-full border border-zinc-800">
      <Clock className="w-4 h-4 text-zinc-400" />
      <span className="text-sm font-mono text-white">{time.time}</span>
      <span className="text-xs text-zinc-400">{time.date}</span>
    </div>
  );
}

export default function App() {
  const [currentTab, setCurrentTab] = useState('agents');
  const [activeModal, setActiveModal] = useState<string | null>(null);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const [selectedContainer, setSelectedContainer] = useState<ContainerItem | null>(null);
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [showLogin, setShowLogin] = useState(true);
  const [loginError, setLoginError] = useState('');
  const [isLoggingIn, setIsLoggingIn] = useState(false);

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
      setAgents(data || []);
    } catch (err) {
      console.error('Failed to fetch agents:', err);
      setAgents([]);
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
    setLoginError('');
    setIsLoggingIn(true);
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
        setLoginError(data.error || 'Invalid username or password');
      }
    } catch (err) {
      setLoginError('Connection failed. Please ensure the server is running.');
    } finally {
      setIsLoggingIn(false);
    }
  };

  if (showLogin) {
    return (
      <LoginScreen
        onLogin={handleLogin}
        error={loginError}
        isLoading={isLoggingIn}
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
                        currentTab === 'logs' ? 'system logs' :
                          currentTab === 'calendar' ? 'calendar' :
                            currentTab === 'allowlist' ? 'allowed users' :
                              currentTab === 'docs' ? 'documentation' :
                                currentTab === 'settings' ? 'settings' : ''}
              </h1>
              <div className="flex items-center gap-3 mt-4 text-sm font-medium text-zinc-500">
                <div className="w-2.5 h-2.5 rounded-full bg-white animate-pulse"></div>
                <span>live system connection active</span>
              </div>
            </div>

            <div className="flex items-center gap-4">
              <SystemClock />

              {currentTab === 'agents' && (
                <button
                  onClick={() => openModal('createAgent')}
                  className="bg-white text-black px-8 py-4 rounded-full font-bold text-sm hover:scale-105 transition-all flex items-center gap-2"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14" /><path d="M12 5v14" /></svg>
                  deploy new agent
                </button>
              )}
            </div>
          </header>

          {currentTab === 'agents' && <AgentsTab agents={agents} openModal={openModal} triggerToast={triggerToast} fetchAgents={fetchAgents} />}
          {currentTab === 'health' && <HealthTab />}
          {currentTab === 'apps' && <AppsTab triggerToast={triggerToast} agents={agents} />}
          {currentTab === 'containers' && <ContainersTab openModal={openModal} triggerToast={triggerToast} />}
          {currentTab === 'logs' && <LogsTab />}
          {currentTab === 'calendar' && <CalendarTab triggerToast={triggerToast} agents={agents} />}
          {currentTab === 'allowlist' && <AllowlistTab triggerToast={triggerToast} />}
          {currentTab === 'docs' && <DocsTab />}
          {currentTab === 'settings' && <SettingsTab triggerToast={triggerToast} onLogout={handleLogout} />}
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

interface LoginScreenProps {
  onLogin: (u: string, p: string) => void;
  error?: string;
  isLoading?: boolean;
}

function LoginScreen({ onLogin, error = '', isLoading = false }: LoginScreenProps) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [localError, setLocalError] = useState(error);

  useEffect(() => {
    setLocalError(error);
  }, [error]);

  const handleLogin = () => {
    if (!username || !password) {
      setLocalError('Username and password are required');
      return;
    }
    setLocalError('');
    onLogin(username, password);
  };

  return (
    <div className="h-screen w-full flex items-center justify-center bg-black">
      <div className="bg-zinc-950 border border-zinc-800 rounded-[3rem] p-12 w-full max-w-md shadow-2xl">
        <div className="text-center mb-8">
          <div className="relative group cursor-pointer flex justify-center mb-8">
            <div className="absolute inset-0 bg-white/10 blur-3xl rounded-full scale-110 animate-pulse" />
            <svg viewBox="0 0 100 100" className="w-24 h-24 relative z-10 drop-shadow-[0_0_30px_rgba(255,255,255,0.4)] transition-transform duration-500 hover:scale-110">
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
          </div>
          <p className="text-zinc-500 mt-2 font-medium tracking-[0.2em] uppercase text-[10px]">Virtual Office System</p>
        </div>

        {localError && (
          <div className="mb-6 p-4 bg-red-500/10 border border-red-500/30 rounded-2xl">
            <p className="text-red-400 text-sm text-center">{localError}</p>
          </div>
        )}

        <div className="space-y-4">
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2 ml-4">Username</label>
            <input
              type="text"
              autoComplete="off"
              placeholder="admin"
              value={username}
              onChange={e => { setUsername(e.target.value); setLocalError(''); }}
              onKeyDown={e => e.key === 'Enter' && handleLogin()}
              className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 placeholder:text-zinc-600"
            />
          </div>
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2 ml-4">Password</label>
            <input
              type="password"
              autoComplete="off"
              placeholder="hermit123"
              value={password}
              onChange={e => { setPassword(e.target.value); setLocalError(''); }}
              onKeyDown={e => e.key === 'Enter' && handleLogin()}
              className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 placeholder:text-zinc-600"
            />
          </div>
          <button
            onClick={handleLogin}
            disabled={isLoading}
            className="w-full bg-white text-black py-4 rounded-full font-bold mt-4 hover:bg-zinc-200 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isLoading ? (
              <>
                <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                Logging in...
              </>
            ) : 'Login'}
          </button>
        </div>

        <div className="mt-6 p-4 bg-zinc-900/50 rounded-2xl border border-zinc-800">
          <p className="text-xs text-zinc-500 text-center">
            <span className="text-yellow-400 font-medium">First time?</span> Default credentials are admin / hermit123.
            Please change your password after login in Settings &gt; Security.
          </p>
        </div>
      </div>
    </div>
  );
}
