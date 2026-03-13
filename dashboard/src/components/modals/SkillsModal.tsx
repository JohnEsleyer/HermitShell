import { useState, useEffect } from 'react';
import { X, Plus, Trash2, FileCode2 } from 'lucide-react';
import { Agent, Skill } from '../../types';

const API_BASE = '';

interface SkillsModalProps {
  agent: Agent;
  onClose: () => void;
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
}

export function SkillsModal({ agent, onClose, triggerToast }: SkillsModalProps) {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [selectedSkillId, setSelectedSkillId] = useState<number>(1);
  const [loading, setLoading] = useState(true);

  const fetchSkills = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/skills`);
      const data = await res.json();
      setSkills(data);
      if (data.length > 0) setSelectedSkillId(data[0].id);
    } catch (err) {
      console.error('Failed to fetch skills:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSkills();
  }, []);

  const selectedSkill = skills.find(s => s.id === selectedSkillId) || skills[0];

  const handleCreate = async () => {
    const newSkill = { title: 'new_skill.md', description: 'Description here', content: '' };
    try {
      const res = await fetch(`${API_BASE}/api/skills`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newSkill),
      });
      const data = await res.json();
      triggerToast('Skill created');
      fetchSkills();
      setSelectedSkillId(data.id);
    } catch (err) {
      triggerToast('Failed to create skill', 'error');
    }
  };

  const handleDelete = async () => {
    if (selectedSkillId === 1) {
      triggerToast('Cannot delete core context.md', 'error');
      return;
    }
    if (!confirm('Delete this skill?')) return;
    try {
      await fetch(`${API_BASE}/api/skills/${selectedSkillId}`, { method: 'DELETE' });
      triggerToast('Skill deleted');
      fetchSkills();
    } catch (err) {
      triggerToast('Failed to delete skill', 'error');
    }
  };

  const handleUpdate = async (field: string, value: string) => {
    if (!selectedSkill) return;
    try {
      await fetch(`${API_BASE}/api/skills/${selectedSkill.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ [field]: value }),
      });
    } catch (err) {
      console.error('Failed to update skill:', err);
    }
  };

  if (loading) {
    return (
      <div className="fixed inset-0 bg-black/85 backdrop-blur-md flex items-center justify-center z-50 p-4 sm:p-6">
        <div className="text-white">Loading skills...</div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 bg-black/85 backdrop-blur-md flex items-center justify-center z-50 p-4 sm:p-6 animate-in fade-in duration-300">
      <div className="bg-zinc-950 border border-zinc-800 w-full max-w-7xl h-[min(88vh,980px)] rounded-[2.5rem] relative flex flex-col shadow-2xl overflow-hidden">
        
        <div className="p-6 border-b border-zinc-800 flex justify-between items-center bg-zinc-900/50">
          <div>
            <h2 className="text-2xl font-bold text-white flex items-center gap-3">
              <FileCode2 className="w-6 h-6" /> Skills: {agent.name}
            </h2>
            <p className="text-sm text-zinc-400 mt-1">Skills are markdown files that extend the brain of the agent. Use &lt;skill&gt;filename.md&lt;/skill&gt; to load them.</p>
          </div>
          <div className="flex gap-3">
            <button onClick={handleCreate} className="px-4 py-2 bg-white text-black rounded-full text-sm font-bold flex items-center gap-2 hover:bg-zinc-200 transition-colors">
              <Plus className="w-4 h-4" /> New Skill
            </button>
            <button onClick={handleDelete} className="px-4 py-2 bg-red-950/50 text-red-400 hover:bg-red-900/50 rounded-full text-sm font-bold flex items-center gap-2 transition-colors">
              <Trash2 className="w-4 h-4" /> Delete
            </button>
            <button onClick={onClose} className="ml-4 w-10 h-10 bg-zinc-800 rounded-full flex items-center justify-center text-zinc-400 hover:text-white transition-all">
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        <div className="flex flex-1 overflow-hidden">
          <div className="w-1/3 border-r border-zinc-800 p-4 overflow-y-auto bg-[#0a0a0a] flex flex-col gap-2">
            {skills.map(skill => (
              <button 
                key={skill.id}
                onClick={() => setSelectedSkillId(skill.id)}
                className={`p-4 rounded-2xl text-left transition-all border ${selectedSkillId === skill.id ? 'bg-zinc-900 border-zinc-700' : 'bg-transparent border-transparent hover:bg-zinc-900/50'}`}
              >
                <div className="font-mono text-sm font-bold text-white mb-1">{skill.title}</div>
                <div className="text-xs text-zinc-500 line-clamp-2">{skill.description}</div>
              </button>
            ))}
          </div>

          <div className="w-2/3 p-6 flex flex-col bg-zinc-950 gap-4">
            <div className="flex gap-4">
              <div className="flex-1">
                <label className="text-xs font-bold text-zinc-500 uppercase tracking-wider mb-2 block">Filename</label>
                <input 
                  value={selectedSkill?.title || ''}
                  onChange={e => handleUpdate('title', e.target.value)}
                  disabled={selectedSkillId === 1}
                  className="w-full bg-black border border-zinc-800 rounded-xl px-4 py-3 text-white font-mono text-sm outline-none focus:border-zinc-600 disabled:opacity-50"
                />
              </div>
              <div className="flex-[2]">
                <label className="text-xs font-bold text-zinc-500 uppercase tracking-wider mb-2 block">Description</label>
                <input 
                  value={selectedSkill?.description || ''}
                  onChange={e => handleUpdate('description', e.target.value)}
                  className="w-full bg-black border border-zinc-800 rounded-xl px-4 py-3 text-white text-sm outline-none focus:border-zinc-600"
                />
              </div>
            </div>
            <div className="flex-1 flex flex-col">
              <label className="text-xs font-bold text-zinc-500 uppercase tracking-wider mb-2 block">Content (Markdown)</label>
              <textarea 
                value={selectedSkill?.content || ''}
                onChange={e => handleUpdate('content', e.target.value)}
                className="flex-1 bg-black border border-zinc-800 rounded-xl p-6 text-zinc-300 font-mono text-sm outline-none focus:border-zinc-600 resize-none"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
