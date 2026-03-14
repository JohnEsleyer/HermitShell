import { Users, Activity, LayoutGrid, Settings, Box, Calendar, Shield, FileText } from 'lucide-react';

interface SidebarProps {
  currentTab: string;
  setCurrentTab: (tab: string) => void;
  onLogout: () => void;
}

export function Sidebar({ currentTab, setCurrentTab, onLogout }: SidebarProps) {
  const tabs = [
    { id: 'agents', name: 'your agents', icon: Users },
    { id: 'health', name: 'system health', icon: Activity },
    { id: 'apps', name: 'published apps', icon: LayoutGrid },
    { id: 'containers', name: 'containers', icon: Box },
    { id: 'logs', name: 'logs', icon: FileText },
    { id: 'calendar', name: 'calendar', icon: Calendar },
    { id: 'allowlist', name: 'allowed users', icon: Shield }
  ];

  return (
    <aside className="w-32 h-full py-8 flex flex-col items-center justify-between z-20">
      <div className="relative group cursor-pointer">
        <svg viewBox="0 0 100 100" className="w-12 h-12 drop-shadow-[0_0_20px_rgba(255,255,255,0.3)] transition-transform duration-300 group-hover:scale-110 group-hover:-translate-y-1">
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
        <span className="absolute left-20 top-4 bg-white text-black text-xs font-bold px-4 py-2 rounded-full opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap z-50 pointer-events-none">hermit os</span>
      </div>

      <nav className="bg-zinc-950 border border-zinc-800/80 rounded-full p-2 flex flex-col gap-2 shadow-2xl">
        {tabs.map(tab => {
          const Icon = tab.icon;
          return (
            <button 
              key={tab.id}
              onClick={() => setCurrentTab(tab.id)} 
              className={`w-14 h-14 rounded-full flex items-center justify-center transition-all duration-300 relative group ${currentTab === tab.id ? 'bg-white text-black' : 'text-zinc-500 hover:bg-zinc-900 hover:text-white'}`}
            >
              <Icon className="w-6 h-6" />
              <span className="absolute left-20 bg-white text-black text-xs font-bold px-4 py-2 rounded-full opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap z-50 pointer-events-none">{tab.name}</span>
            </button>
          );
        })}
      </nav>

      <button onClick={() => setCurrentTab('settings')} className={`w-14 h-14 rounded-full border flex items-center justify-center transition-all group relative ${currentTab === 'settings' ? 'bg-white text-black border-white' : 'bg-zinc-950 border-zinc-800 text-zinc-500 hover:bg-zinc-900 hover:text-white'}`}>
        <Settings className="w-6 h-6" />
        <span className="absolute left-20 bg-white text-black text-xs font-bold px-4 py-2 rounded-full opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap z-50 pointer-events-none">settings</span>
      </button>
    </aside>
  );
}
