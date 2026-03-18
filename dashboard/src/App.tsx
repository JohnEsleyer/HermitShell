// Hermit Dashboard - React Frontend
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

  // handleLogin authenticates dashboard users and returns inline form errors for failed logins.
  // Docs: See docs/authentication.md for the login API contract and first-login password change flow.
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
          triggerToast('Please change your credentials in the settings dashboard before continuing.', 'info');
        }
        return null;
      }

      return data.error || 'Invalid username or password.';
    } catch (err) {
      return 'Unable to reach the HermitShell server. Check that the service is running and try again.';
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
                  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14"/><path d="M12 5v14"/></svg>
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

// LoginScreen presents first-time credentials guidance and inline authentication errors.
// Docs: See docs/authentication.md for the required first-login password rotation policy.
function LoginScreen({ onLogin }: { onLogin: (u: string, p: string) => Promise<string | null> }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [errorMessage, setErrorMessage] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  // submitLogin keeps login feedback visible inside the form so failed authentication is actionable.
  // Docs: See docs/frontend-backend-communication.md for how the dashboard calls the auth API.
  const submitLogin = async () => {
    setErrorMessage('');

    if (!username.trim() || !password.trim()) {
      setErrorMessage('Enter both the username and password to sign in.');
      return;
    }

    setIsSubmitting(true);
    const error = await onLogin(username.trim(), password);
    setIsSubmitting(false);

    if (error) {
      setErrorMessage(error);
    }
  };

  return (
    <div className="h-screen w-full flex items-center justify-center bg-black">
      <div className="bg-zinc-950 border border-zinc-800 rounded-[3rem] p-12 w-full max-w-md shadow-2xl">
        <div className="text-center mb-8">
          <svg viewBox="0 0 100 100" className="w-16 h-16 mx-auto mb-4 drop-shadow-[0_0_20px_rgba(255,255,255,0.3)]">
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
          <h1 className="text-4xl font-black tracking-tighter">HERMITSHELL</h1>
          <p className="text-zinc-500 mt-2">Agent Orchestration System</p>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2 ml-4">Username</label>
            <input 
              type="text" 
              autoComplete="off"
              placeholder="admin"
              value={username}
              onChange={e => setUsername(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') void submitLogin(); }}
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
              onChange={e => setPassword(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') void submitLogin(); }}
              className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 placeholder:text-zinc-600"
            />
          </div>

          <div className="rounded-3xl border border-amber-500/30 bg-amber-500/10 px-5 py-4 text-sm text-amber-100">
            <p className="font-semibold uppercase tracking-wide text-amber-200">First-time setup</p>
            <p className="mt-2">Default credentials: <span className="font-mono text-white">admin / hermit123</span>.</p>
            <p className="mt-2 text-amber-50">Changing these default credentials in the settings dashboard is required before regular use.</p>
          </div>

          {errorMessage && (
            <div className="rounded-3xl border border-red-500/30 bg-red-500/10 px-5 py-4 text-sm text-red-200">
              {errorMessage}
            </div>
          )}

          <button 
            onClick={() => void submitLogin()}
            disabled={isSubmitting}
            className="w-full bg-white text-black py-4 rounded-full font-bold mt-4 hover:bg-zinc-200 transition-colors disabled:cursor-not-allowed disabled:bg-zinc-300"
          >
            {isSubmitting ? 'Signing in...' : 'Login'}
          </button>
        </div>
      </div>
    </div>
  );
}
