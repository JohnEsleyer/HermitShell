import { useState } from 'react';
import { 
  MessageSquare, Terminal, FileCode, Database, Container, 
  Globe, Clock, Brain, Cpu, Zap, ArrowRight, ChevronDown, ChevronUp,
  Bot, MessageCircle, Calendar, Bell, Send, Server, Network,
  BookOpen, FileText
} from 'lucide-react';

type DocSection = 'overview' | 'xml' | 'docker' | 'database' | 'flow';

interface Section {
  id: DocSection;
  title: string;
  icon: typeof MessageSquare;
  color: string;
}

const sections: Section[] = [
  { id: 'overview', title: 'System Overview', icon: Brain, color: 'from-purple-500 to-pink-500' },
  { id: 'xml', title: 'XML Tags Reference', icon: FileCode, color: 'from-blue-500 to-cyan-500' },
  { id: 'docker', title: 'Docker Container', icon: Container, color: 'from-orange-500 to-red-500' },
  { id: 'database', title: 'Database Schema', icon: Database, color: 'from-green-500 to-emerald-500' },
  { id: 'flow', title: 'Message Flow', icon: Network, color: 'from-indigo-500 to-purple-500' },
];

function DocCard({ title, icon: Icon, gradient, children }: { title: string; icon: typeof MessageSquare; gradient: string; children: React.ReactNode }) {
  return (
    <div className="bg-zinc-900/50 border border-zinc-800 rounded-2xl overflow-hidden">
      <div className={`bg-gradient-to-r ${gradient} p-4`}>
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-white/20 rounded-xl flex items-center justify-center">
            <Icon className="w-5 h-5 text-white" />
          </div>
          <h3 className="text-xl font-bold text-white">{title}</h3>
        </div>
      </div>
      <div className="p-6">
        {children}
      </div>
    </div>
  );
}

function Collapsible({ title, children, defaultOpen = false }: { title: string; children: React.ReactNode; defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  return (
    <div className="border border-zinc-800 rounded-xl overflow-hidden">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between p-4 bg-zinc-800/50 hover:bg-zinc-800 transition-colors"
      >
        <span className="font-medium text-white">{title}</span>
        {isOpen ? <ChevronUp className="w-5 h-5 text-zinc-400" /> : <ChevronDown className="w-5 h-5 text-zinc-400" />}
      </button>
      {isOpen && <div className="p-4 bg-zinc-900/30">{children}</div>}
    </div>
  );
}

function Tag({ name, color = 'blue' }: { name: string; color?: string }) {
  const colors: Record<string, string> = {
    blue: 'bg-blue-500/20 text-blue-300 border-blue-500/30',
    green: 'bg-green-500/20 text-green-300 border-green-500/30',
    purple: 'bg-purple-500/20 text-purple-300 border-purple-500/30',
    orange: 'bg-orange-500/20 text-orange-300 border-orange-500/30',
    pink: 'bg-pink-500/20 text-pink-300 border-pink-500/30',
    cyan: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/30',
  };
  return <span className={`px-2 py-0.5 rounded text-xs font-mono border ${colors[color] || colors.blue}`}>{name}</span>;
}

function Arrow({ direction = 'right' }: { direction?: 'right' | 'down' | 'left' }) {
  const rotations = { right: 0, down: 90, left: 180 };
  return <ArrowRight className="w-5 h-5 text-zinc-600" style={{ transform: `rotate(${rotations[direction]}deg)` }} />;
}

export function DocsTab() {
  const [activeSection, setActiveSection] = useState<DocSection>('overview');

  return (
    <div className="flex-1 flex gap-6">
      {/* Sidebar Navigation */}
      <div className="w-64 shrink-0">
        <div className="sticky top-0 space-y-2">
          {sections.map(section => {
            const Icon = section.icon;
            return (
              <button
                key={section.id}
                onClick={() => setActiveSection(section.id)}
                className={`w-full flex items-center gap-3 p-4 rounded-xl transition-all ${
                  activeSection === section.id
                    ? 'bg-white text-black'
                    : 'bg-zinc-900/50 text-zinc-400 hover:bg-zinc-800 hover:text-white'
                }`}
              >
                <Icon className="w-5 h-5" />
                <span className="font-medium">{section.title}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto space-y-6 pr-4">
        {activeSection === 'overview' && (
          <>
            <div className="bg-gradient-to-r from-purple-500/20 to-pink-500/20 border border-purple-500/30 rounded-2xl p-6">
              <h2 className="text-3xl font-black mb-4 bg-gradient-to-r from-purple-400 to-pink-400 bg-clip-text text-transparent">
                HermitShell AI Agent OS
              </h2>
              <p className="text-zinc-300">
                A lightweight AI agent orchestrator built with Go, designed for efficient VPS environments. 
                Each AI agent runs in its own isolated Docker container with dedicated workspace.
              </p>
            </div>

            <div className="grid grid-cols-3 gap-4">
              {[
                { icon: Bot, title: 'AI Agents', desc: 'Autonomous agents with LLM capabilities', color: 'bg-blue-500' },
                { icon: Container, title: 'Containers', desc: 'Isolated Docker workspaces', color: 'bg-orange-500' },
                { icon: Globe, title: 'Telegram', desc: 'User interaction via Telegram Bot API', color: 'bg-cyan-500' },
                { icon: Calendar, title: 'Scheduler', desc: 'Event-driven agent automation', color: 'bg-green-500' },
                { icon: Database, title: 'SQLite', desc: 'Persistent storage for all data', color: 'bg-purple-500' },
                { icon: Cpu, title: 'Dashboard', desc: 'Real-time monitoring & control', color: 'bg-pink-500' },
              ].map(item => (
                <div key={item.title} className="bg-zinc-900/50 border border-zinc-800 p-4 rounded-xl">
                  <div className={`w-10 h-10 ${item.color} rounded-lg mb-3 flex items-center justify-center`}>
                    <item.icon className="w-5 h-5 text-white" />
                  </div>
                  <h3 className="font-bold text-white mb-1">{item.title}</h3>
                  <p className="text-xs text-zinc-500">{item.desc}</p>
                </div>
              ))}
            </div>

            <DocCard title="Architecture" icon={Brain} gradient="from-purple-500 to-pink-500">
              <div className="space-y-4">
                <div className="flex items-center justify-center gap-4 flex-wrap">
                  <div className="bg-blue-500/20 border border-blue-500/30 px-4 py-2 rounded-xl">
                    <span className="text-blue-300 font-medium">Telegram User</span>
                  </div>
                  <Arrow />
                  <div className="bg-green-500/20 border border-green-500/30 px-4 py-2 rounded-xl">
                    <span className="text-green-300 font-medium">HermitShell Server</span>
                  </div>
                  <Arrow />
                  <div className="bg-purple-500/20 border border-purple-500/30 px-4 py-2 rounded-xl">
                    <span className="text-purple-300 font-medium">LLM</span>
                  </div>
                  <Arrow />
                  <div className="bg-orange-500/20 border border-orange-500/30 px-4 py-2 rounded-xl">
                    <span className="text-orange-300 font-medium">Docker Agent</span>
                  </div>
                </div>
                
                <div className="grid grid-cols-2 gap-4 mt-4">
                  <div className="bg-zinc-800/50 p-4 rounded-xl">
                    <h4 className="font-bold text-white mb-2 flex items-center gap-2">
                      <Zap className="w-4 h-4 text-yellow-400" /> What Agents Can Do
                    </h4>
                    <ul className="text-sm text-zinc-400 space-y-1">
                      <li>• Execute shell commands</li>
                      <li>• Send Telegram messages</li>
                      <li>• Send files to users</li>
                      <li>• Publish web apps</li>
                      <li>• Schedule reminders</li>
                    </ul>
                  </div>
                  <div className="bg-zinc-800/50 p-4 rounded-xl">
                    <h4 className="font-bold text-white mb-2 flex items-center gap-2">
                      <Server className="w-4 h-4 text-blue-400" /> What's Stored
                    </h4>
                    <ul className="text-sm text-zinc-400 space-y-1">
                      <li>• Agent configurations</li>
                      <li>• Conversation history</li>
                      <li>• Audit logs</li>
                      <li>• Scheduled events</li>
                      <li>• Skills & knowledge</li>
                    </ul>
                  </div>
                </div>
              </div>
            </DocCard>
          </>
        )}

        {activeSection === 'xml' && (
          <>
            <div className="bg-gradient-to-r from-blue-500/20 to-cyan-500/20 border border-blue-500/30 rounded-2xl p-6 mb-6">
              <h2 className="text-2xl font-bold text-white mb-2">XML Contract Tags</h2>
              <p className="text-zinc-400 text-sm">
                AI agents use XML tags to perform actions. Only content inside these tags is processed by the system.
              </p>
            </div>

            <DocCard title="<message>" icon={MessageCircle} gradient="from-blue-500 to-cyan-500">
              <p className="text-zinc-400 mb-4">Sends a message to the Telegram user. <strong className="text-white">Required for all visible output.</strong></p>
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm">
                <span className="text-pink-400">{'<message>'}</span>Hello! How can I help you?<span className="text-pink-400">{'</message>'}</span>
              </div>
            </DocCard>

            <DocCard title="<thought>" icon={Brain} gradient="from-purple-500 to-pink-500">
              <p className="text-zinc-400 mb-4">Internal reasoning - never sent to user. Used for tracking agent thinking.</p>
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm">
                <span className="text-pink-400">{'<thought>'}</span>The user wants a file, I need to create it first.<span className="text-pink-400">{'</thought>'}</span>
              </div>
            </DocCard>

            <DocCard title="<terminal>" icon={Terminal} gradient="from-orange-500 to-red-500">
              <p className="text-zinc-400 mb-4">Executes a shell command inside the agent's Docker container.</p>
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm">
                <span className="text-pink-400">{'<terminal>'}</span>echo "hello" {'>'} /app/workspace/out/hello.txt<span className="text-pink-400">{'</terminal>'}</span>
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                <Tag name="workspace" color="orange" />
                <Tag name="read files" color="orange" />
                <Tag name="execute code" color="orange" />
              </div>
            </DocCard>

            <DocCard title="<give>" icon={Send} gradient="from-green-500 to-emerald-500">
              <p className="text-zinc-400 mb-4">Send a file from the container's <code className="text-white">/app/workspace/out/</code> folder to the user via Telegram.</p>
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm">
                <span className="text-pink-400">{'<give>'}</span>report.pdf<span className="text-pink-400">{'</give>'}</span>
              </div>
              <p className="text-xs text-zinc-500 mt-3">The file must exist in <code>/app/workspace/out/</code> within the container.</p>
            </DocCard>

            <DocCard title="<app>" icon={Globe} gradient="from-cyan-500 to-blue-500">
              <p className="text-zinc-400 mb-4">Create a web application with HTML, CSS, and JavaScript. The system automatically creates the file structure.</p>
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm overflow-x-auto">
                <span className="text-pink-400">&lt;app name="myapp"&gt;</span><br/>
                <span className="text-blue-300">  &lt;html&gt;</span><br/>
                <span className="text-white">    &lt;button&gt;Click me&lt;/button&gt;</span><br/>
                <span className="text-blue-300">  &lt;/html&gt;</span><br/>
                <span className="text-green-300">  &lt;style&gt;</span><br/>
                <span className="text-white">    button {'{'} padding: 10px; {'}'}</span><br/>
                <span className="text-green-300">  &lt;/style&gt;</span><br/>
                <span className="text-yellow-300">  &lt;script&gt;</span><br/>
                <span className="text-white">    // JS code here</span><br/>
                <span className="text-yellow-300">  &lt;/script&gt;</span><br/>
                <span className="text-pink-400">&lt;/app&gt;</span>
              </div>
              <p className="text-xs text-zinc-500 mt-3">Creates /app/workspace/apps/myapp/index.html and publishes via Traefik.</p>
            </DocCard>

            <DocCard title="<calendar>" icon={Calendar} gradient="from-yellow-500 to-orange-500">
              <p className="text-zinc-400 mb-4">Schedule a reminder or event for the future.</p>
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm">
                <span className="text-pink-400">{'<calendar>'}</span>{'\n'}
                {'  '}<span className="text-blue-300">{'<datetime>'}</span>2026-03-20T09:00:00<span className="text-blue-300">{'</datetime>'}</span>{'\n'}
                {'  '}<span className="text-green-300">{'<prompt>'}</span>Time to wake up!<span className="text-green-300">{'</prompt>'}</span>{'\n'}
                <span className="text-pink-400">{'</calendar>'}</span>
              </div>
            </DocCard>

            <DocCard title="<system>" icon={Zap} gradient="from-cyan-500 to-blue-500">
              <p className="text-zinc-400 mb-4">Request runtime information from the system.</p>
              <div className="space-y-3">
                <div className="bg-zinc-950 p-3 rounded-xl font-mono text-sm">
                  <span className="text-pink-400">{'<system>'}</span>time<span className="text-pink-400">{'</system>'}</span>
                  <span className="text-z-500 ml-2">→ Returns current time</span>
                </div>
                <div className="bg-zinc-950 p-3 rounded-xl font-mono text-sm">
                  <span className="text-pink-400">{'<system>'}</span>date<span className="text-pink-400">{'</system>'}</span>
                  <span className="text-z-500 ml-2">→ Returns current date</span>
                </div>
                <div className="bg-zinc-950 p-3 rounded-xl font-mono text-sm">
                  <span className="text-pink-400">{'<system>'}</span>memory<span className="text-pink-400">{'</system>'}</span>
                  <span className="text-z-500 ml-2">→ Returns memory usage</span>
                </div>
              </div>
            </DocCard>

            <DocCard title="<skill>" icon={BookOpen} gradient="from-indigo-500 to-purple-500">
              <p className="text-zinc-400 mb-4">Load a skill file into the agent's context.</p>
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm">
                <span className="text-pink-400">{'<skill>'}</span>reminder.md<span className="text-pink-400">{'</skill>'}</span>
              </div>
            </DocCard>
          </>
        )}

        {activeSection === 'docker' && (
          <>
            <DocCard title="Container Architecture" icon={Container} gradient="from-orange-500 to-red-500">
              <p className="text-zinc-400 mb-4">Each AI agent runs in an isolated Docker container with its own filesystem.</p>
              
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm mb-4">
                <div className="text-zinc-500">hermit-agent:latest</div>
                <hr className="border-zinc-800 my-2" />
                <div className="text-zinc-400">/app/workspace/</div>
                <div className="pl-4 text-blue-300">├── work/     # Scratchpad, scripts</div>
                <div className="pl-4 text-green-300">├── in/       # User input files</div>
                <div className="pl-4 text-purple-300">├── out/      # Files to give user</div>
                <div className="pl-4 text-orange-300">└── apps/    # Published web apps</div>
              </div>
            </DocCard>

            <DocCard title="Container Lifecycle" icon={Cpu} gradient="from-red-500 to-pink-500">
              <div className="space-y-3">
                {[
                  { status: 'Created', desc: 'Container built from hermit-agent image', color: 'blue' },
                  { status: 'Started', desc: 'Container running, ready for commands', color: 'green' },
                  { status: 'Idle', desc: 'No activity for 5 minutes, auto-stopped', color: 'yellow' },
                  { status: 'Reset', desc: 'Container reset to clean state', color: 'orange' },
                ].map(item => (
                  <div key={item.status} className="flex items-center gap-4">
                    <div className={`w-3 h-3 rounded-full bg-${item.color}-500`} />
                    <div>
                      <span className="font-medium text-white">{item.status}</span>
                      <span className="text-zinc-500 text-sm ml-2">- {item.desc}</span>
                    </div>
                  </div>
                ))}
              </div>
            </DocCard>

            <DocCard title="File Operations" icon={FileCode} gradient="from-amber-500 to-yellow-500">
              <div className="space-y-4">
                <div className="bg-zinc-800/50 p-4 rounded-xl">
                  <h4 className="font-bold text-white mb-2">Reading Files</h4>
                  <p className="text-sm text-zinc-400">Files in /app/workspace/in/ are available to the agent for reading.</p>
                </div>
                <div className="bg-zinc-800/50 p-4 rounded-xl">
                  <h4 className="font-bold text-white mb-2">Writing Output</h4>
                  <p className="text-sm text-zinc-400">Files written to /app/workspace/out/ can be sent to users via GIVE action.</p>
                </div>
                <div className="bg-zinc-800/50 p-4 rounded-xl">
                  <h4 className="font-bold text-white mb-2">Web Apps</h4>
                  <p className="text-sm text-zinc-400">HTML/CSS/JS in /app/workspace/apps/ can be published via <code className="text-green-400">&lt;app&gt;</code> tag.</p>
                </div>
              </div>
            </DocCard>

            <DocCard title="Agent Statistics" icon={Zap} gradient="from-yellow-500 to-orange-500">
              <p className="text-zinc-400 mb-4">Each agent tracks usage metrics for monitoring and cost estimation.</p>
              <div className="space-y-3">
                <div className="bg-zinc-950 p-3 rounded-xl">
                  <span className="text-white font-medium">LLM API Calls</span>
                  <p className="text-xs text-zinc-500 mt-1">Increments by 1 on every LLM API request</p>
                </div>
                <div className="bg-zinc-950 p-3 rounded-xl">
                  <span className="text-white font-medium">Context Window</span>
                  <p className="text-xs text-zinc-500 mt-1">Maximum token limit for the model (e.g., 1M for Gemini)</p>
                </div>
                <div className="bg-zinc-950 p-3 rounded-xl">
                  <span className="text-white font-medium">Word Count</span>
                  <p className="text-xs text-zinc-500 mt-1">Total words in conversation history</p>
                </div>
                <div className="bg-zinc-950 p-3 rounded-xl">
                  <span className="text-white font-medium">Estimated Cost</span>
                  <p className="text-xs text-zinc-500 mt-1">Cumulative cost based on token usage</p>
                </div>
              </div>
              <p className="text-xs text-zinc-500 mt-4">View in agent card on dashboard or via <code className="text-green-400">/status</code> Telegram command.</p>
            </DocCard>
          </>
        )}

        {activeSection === 'database' && (
          <>
            <DocCard title="Database Schema" icon={Database} gradient="from-green-500 to-emerald-500">
              <p className="text-zinc-400 mb-4">HermitShell uses SQLite for persistent storage. All data is stored in hermit.db.</p>
              
              <div className="space-y-4">
                <Collapsible title="agents" defaultOpen>
                  <div className="font-mono text-sm text-zinc-300 space-y-1">
                    <div>id, name, role, personality</div>
                    <div>provider, model, system_prompt</div>
                    <div>telegram_id, telegram_token</div>
                    <div>profile_pic, banner_url</div>
                    <div>container_id, status, active</div>
                    <div>llm_api_calls, context_window</div>
                    <div>created_at, updated_at</div>
                  </div>
                </Collapsible>
                
                <Collapsible title="audit_logs">
                  <div className="font-mono text-sm text-zinc-300 space-y-1">
                    <div>id, agent_id, user_id</div>
                    <div>action, details</div>
                    <div>created_at</div>
                  </div>
                </Collapsible>
                
                <Collapsible title="history">
                  <div className="font-mono text-sm text-zinc-300 space-y-1">
                    <div>id, agent_id, user_id</div>
                    <div>role (user/assistant/system)</div>
                    <div>content, created_at</div>
                  </div>
                </Collapsible>
                
                <Collapsible title="calendar">
                  <div className="font-mono text-sm text-zinc-300 space-y-1">
                    <div>id, agent_id, date, time</div>
                    <div>prompt, executed</div>
                    <div>created_at</div>
                  </div>
                </Collapsible>
                
                <Collapsible title="skills">
                  <div className="font-mono text-sm text-zinc-300 space-y-1">
                    <div>id, agent_id, title</div>
                    <div>description, content</div>
                    <div>created_at</div>
                  </div>
                </Collapsible>
                
                <Collapsible title="allowlist">
                  <div className="font-mono text-sm text-zinc-300 space-y-1">
                    <div>id, telegram_user_id</div>
                    <div>friendly_name, notes</div>
                    <div>created_at</div>
                  </div>
                </Collapsible>
              </div>
            </DocCard>

            <DocCard title="Log Types" icon={FileText} gradient="from-teal-500 to-cyan-500">
              <p className="text-zinc-400 mb-4">Audit logs are categorized by prefix for filtering.</p>
              
              <div className="grid grid-cols-2 gap-3">
                {[
                  { prefix: 'system.*', desc: 'System actions', color: 'emerald' },
                  { prefix: 'agent.*', desc: 'Agent behavior', color: 'yellow' },
                  { prefix: 'docker.*', desc: 'Container events', color: 'blue' },
                  { prefix: 'tunnel.*', desc: 'Network tunnels', color: 'purple' },
                  { prefix: 'llm_*', desc: 'LLM requests', color: 'pink' },
                ].map(item => (
                  <div key={item.prefix} className="bg-zinc-800/50 p-3 rounded-lg">
                    <Tag name={item.prefix} color={item.color} />
                    <p className="text-xs text-zinc-500 mt-1">{item.desc}</p>
                  </div>
                ))}
              </div>
            </DocCard>
          </>
        )}

        {activeSection === 'flow' && (
          <>
            <DocCard title="Message Processing Flow" icon={Network} gradient="from-indigo-500 to-purple-500">
              <div className="relative">
                <div className="space-y-6">
                  {[
                    { step: 1, title: 'Telegram Message', desc: 'User sends message via Telegram', icon: Send, color: 'cyan' },
                    { step: 2, title: 'Webhook Received', desc: 'Server receives at /webhook/:agentId', icon: Globe, color: 'blue' },
                    { step: 3, title: 'Container Started', desc: 'Docker container starts if not running', icon: Container, color: 'orange' },
                    { step: 4, title: 'Context Loaded', desc: 'System prompt + history + skills sent to LLM', icon: Brain, color: 'purple' },
                    { step: 5, title: 'LLM Processing', desc: 'AI generates XML response', icon: Cpu, color: 'pink' },
                    { step: 6, title: 'XML Parsing', desc: 'System extracts and executes actions', icon: FileCode, color: 'green' },
                    { step: 7, title: 'Action Execution', desc: 'Commands run, messages sent, files delivered', icon: Zap, color: 'yellow' },
                    { step: 8, title: 'History Saved', desc: 'Full exchange stored in database', icon: Database, color: 'emerald' },
                  ].map(item => (
                    <div key={item.step} className="flex gap-4">
                      <div className="flex flex-col items-center">
                        <div className={`w-10 h-10 bg-${item.color}-500 rounded-full flex items-center justify-center font-bold text-white`}>
                          {item.step}
                        </div>
                        {item.step < 8 && <div className="w-0.5 h-8 bg-zinc-700 mt-2" />}
                      </div>
                      <div className="flex-1 pb-6">
                        <div className="flex items-center gap-2">
                          <item.icon className={`w-4 h-4 text-${item.color}-400`} />
                          <span className="font-bold text-white">{item.title}</span>
                        </div>
                        <p className="text-sm text-zinc-500">{item.desc}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </DocCard>

            <DocCard title="Takeover Mode" icon={Terminal} gradient="from-red-500 to-orange-500">
              <p className="text-zinc-400 mb-4">
                Users can use /takeover command to directly control the agent container with XML commands.
              </p>
              <div className="bg-zinc-950 p-4 rounded-xl font-mono text-sm">
                <div className="text-zinc-500">User types:</div>
                <div className="text-white">/takeover</div>
                <div className="text-zinc-500 mt-2">Then sends XML directly:</div>
                <div className="text-blue-300">{'<terminal>'}ls -la{'</terminal>'}</div>
                <div className="text-green-300">{'<message>'}Here are the files!{'</message>'}</div>
              </div>
            </DocCard>
          </>
        )}
      </div>
    </div>
  );
}
