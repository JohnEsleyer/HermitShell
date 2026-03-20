import { Users, Activity, LayoutGrid, Settings, Box, Calendar, Shield, FileText, ChevronRight, BookOpen, LogOut } from 'lucide-react';

interface SidebarProps {
  currentTab: string;
  setCurrentTab: (tab: string) => void;
  onLogout: () => void;
}

export function Sidebar({ currentTab, setCurrentTab, onLogout }: SidebarProps) {
  const tabs = [
    { id: 'agents', name: 'Agents', icon: Users, description: 'Manage your AI agents' },
    { id: 'health', name: 'System Health', icon: Activity, description: 'CPU, RAM, Disk usage' },
    { id: 'apps', name: 'Published Apps', icon: LayoutGrid, description: 'Deployed web apps' },
    { id: 'containers', name: 'Containers', icon: Box, description: 'Docker containers' },
    { id: 'logs', name: 'Logs', icon: FileText, description: 'System & agent logs' },
    { id: 'calendar', name: 'Calendar', icon: Calendar, description: 'Scheduled events' },
    { id: 'allowlist', name: 'Allowed Users', icon: Shield, description: 'Telegram access' },
    { id: 'docs', name: 'Docs', icon: BookOpen, description: 'Documentation & guides' }
  ];

  return (
    <aside className="w-56 h-full flex flex-col z-20 bg-zinc-950/50 border-r border-zinc-800/50">
      {/* Logo */}
      <div className="py-4 px-6 border-b border-zinc-800/50">
        <div className="relative group cursor-pointer flex justify-center">
          <div className="absolute inset-0 bg-white/10 blur-xl rounded-full scale-125 animate-pulse" />
          <svg viewBox="0 0 100 100" className="w-8 h-8 relative z-10 drop-shadow-[0_0_15px_rgba(255,255,255,0.4)] transition-transform duration-500 group-hover:scale-110">
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
      </div>

      {/* Navigation - Scrollable */}
      <nav className="flex-1 overflow-y-auto py-4 px-3">
        <div className="space-y-1">
          {tabs.map(tab => {
            const Icon = tab.icon;
            const isActive = currentTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setCurrentTab(tab.id)}
                className={`w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 group ${isActive ? 'bg-white text-black' : 'text-zinc-400 hover:bg-zinc-900 hover:text-white'}`}
              >
                <Icon className="w-5 h-5 shrink-0" />
                <div className="text-left">
                  <div className={`text-sm font-bold ${isActive ? 'text-black' : ''}`}>{tab.name}</div>
                  <div className={`text-[10px] ${isActive ? 'text-zinc-600' : 'text-zinc-600 group-hover:text-zinc-500'}`}>{tab.description}</div>
                </div>
                <ChevronRight className={`w-4 h-4 ml-auto opacity-0 group-hover:opacity-50 transition-opacity ${isActive ? 'hidden' : ''}`} />
              </button>
            );
          })}
        </div>
      </nav>

      {/* Settings & Logout */}
      <div className="p-3 border-t border-zinc-800/50 space-y-1">
        <button
          onClick={() => setCurrentTab('settings')}
          className={`w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 group ${currentTab === 'settings' ? 'bg-white text-black' : 'text-zinc-400 hover:bg-zinc-900 hover:text-white'}`}
        >
          <Settings className="w-5 h-5" />
          <span className="text-sm font-bold">Settings</span>
        </button>
        <button
          onClick={onLogout}
          className="w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 text-red-500/70 hover:bg-red-500/10 hover:text-red-400 group"
        >
          <LogOut className="w-5 h-5 shrink-0" />
          <div className="text-left font-bold text-sm">Logout</div>
        </button>
      </div>
    </aside>
  );
}
