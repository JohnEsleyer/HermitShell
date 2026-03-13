import { useState } from 'react';
import { Settings2, UploadCloud, X } from 'lucide-react';
import { Agent } from '../../types';

const API_BASE = '';

interface ConfigureModalProps {
  agent: Agent;
  onClose: () => void;
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
}

export function ConfigureModal({ agent, onClose, triggerToast }: ConfigureModalProps) {
  const [formData, setFormData] = useState({
    name: agent.name,
    role: agent.role,
    personality: agent.personality || '',
    provider: agent.provider || 'openrouter',
    profilePic: agent.profilePic || '',
    bannerUrl: agent.bannerUrl || '',
  });

  const uploadImage = async (file: File, type: 'profile' | 'banner') => {
    const body = new FormData();
    body.append('image', file);
    body.append('type', type);

    try {
      const res = await fetch(`${API_BASE}/api/images/upload`, { method: 'POST', body });
      const data = await res.json();
      if (!res.ok || !data.url) throw new Error(data.error || 'Upload failed');
      if (type === 'profile') setFormData(prev => ({ ...prev, profilePic: data.url }));
      if (type === 'banner') setFormData(prev => ({ ...prev, bannerUrl: data.url }));
      triggerToast('Image uploaded');
    } catch {
      triggerToast('Failed to upload image', 'error');
    }
  };

  const handleSave = async () => {
    try {
      await fetch(`${API_BASE}/api/agents/${agent.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData),
      });
      triggerToast('Agent configuration saved');
      onClose();
    } catch (err) {
      triggerToast('Failed to save', 'error');
    }
  };

  return (
    <div className="fixed inset-0 bg-black/85 backdrop-blur-md flex items-center justify-center z-50 p-4 sm:p-6 animate-in fade-in duration-300">
      <div className="bg-zinc-950 border border-zinc-800 w-full max-w-3xl rounded-[2.5rem] p-6 sm:p-10 relative flex flex-col shadow-2xl max-h-[90vh] overflow-y-auto">
        <button onClick={onClose} className="absolute top-6 right-6 sm:top-8 sm:right-8 w-11 h-11 bg-zinc-900 rounded-full flex items-center justify-center text-zinc-500 hover:text-white transition-all">
          <X className="w-6 h-6" />
        </button>

        <h2 className="text-3xl font-black text-white mb-2 flex items-center gap-3 lowercase">
          <Settings2 className="w-8 h-8" /> configure {agent.name}
        </h2>
        <p className="text-zinc-500 text-sm lowercase mb-8 border-b border-zinc-800 pb-6">update agent settings and connections</p>

        <div className="space-y-6 flex-1">
          <div>
            <label className="block text-sm text-zinc-400 mb-2 ml-4">Agent Name</label>
            <input type="text" value={formData.name} onChange={e => setFormData({ ...formData, name: e.target.value })} className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors" />
          </div>
          <div>
            <label className="block text-sm text-zinc-400 mb-2 ml-4">Role</label>
            <input type="text" value={formData.role} onChange={e => setFormData({ ...formData, role: e.target.value })} className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors" />
          </div>
          <div>
            <label className="block text-sm text-zinc-400 mb-2 ml-4">Personality</label>
            <input type="text" value={formData.personality} onChange={e => setFormData({ ...formData, personality: e.target.value })} className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors" />
          </div>
          <div>
            <label className="block text-sm text-zinc-400 mb-2 ml-4">LLM Provider</label>
            <select value={formData.provider} onChange={e => setFormData({ ...formData, provider: e.target.value })} className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors appearance-none">
              <option value="openrouter">OpenRouter (Free Models Only)</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Gemini</option>
            </select>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="bg-black border border-zinc-800 rounded-2xl p-4">
              <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-2">Profile image URL</label>
              <input type="text" value={formData.profilePic} onChange={e => setFormData({ ...formData, profilePic: e.target.value })} className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-2.5 text-white outline-none focus:border-zinc-500 transition-colors text-sm" />
              <label className="mt-3 inline-flex items-center gap-2 cursor-pointer text-xs text-zinc-300 hover:text-white">
                <UploadCloud className="w-4 h-4" /> Upload profile
                <input type="file" accept="image/*" className="hidden" onChange={e => e.target.files?.[0] && uploadImage(e.target.files[0], 'profile')} />
              </label>
            </div>
            <div className="bg-black border border-zinc-800 rounded-2xl p-4">
              <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-2">Banner image URL</label>
              <input type="text" value={formData.bannerUrl} onChange={e => setFormData({ ...formData, bannerUrl: e.target.value })} className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-2.5 text-white outline-none focus:border-zinc-500 transition-colors text-sm" />
              <label className="mt-3 inline-flex items-center gap-2 cursor-pointer text-xs text-zinc-300 hover:text-white">
                <UploadCloud className="w-4 h-4" /> Upload banner
                <input type="file" accept="image/*" className="hidden" onChange={e => e.target.files?.[0] && uploadImage(e.target.files[0], 'banner')} />
              </label>
            </div>
          </div>
        </div>

        <div className="flex justify-end pt-10 mt-auto">
          <button onClick={handleSave} className="bg-white text-black px-10 py-4 rounded-full font-bold hover:bg-zinc-200 transition-colors">save changes</button>
        </div>
      </div>
    </div>
  );
}
