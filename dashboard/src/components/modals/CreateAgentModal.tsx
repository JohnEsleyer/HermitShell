import { useMemo, useState, useEffect } from 'react';
import { ImagePlus, UploadCloud, X } from 'lucide-react';

interface CreateAgentModalProps {
  onClose: () => void;
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
  fetchAgents: () => void;
}

const API_BASE = '';

export function CreateAgentModal({ onClose, triggerToast, fetchAgents }: CreateAgentModalProps) {
  const [step, setStep] = useState(1);
  const [formData, setFormData] = useState({
    name: '',
    role: '',
    personality: '',
    provider: 'openrouter',
    profilePic: '',
    bannerUrl: '',
    model: '',
    allowedUsers: '',
    platform: 'telegram',
  });
  const [allowlist, setAllowlist] = useState<{ friendlyName: string, telegramUserId: string }[]>([]);

  useEffect(() => {
    fetch(`${API_BASE}/api/allowlist`)
      .then(res => res.json())
      .then(data => setAllowlist(data || []))
      .catch(err => console.error('Failed to fetch allowlist:', err));
  }, []);
  const [telegramData, setTelegramData] = useState({
    botToken: '',
    allowedUserId: '',
  });
  const [verifyCode, setVerifyCode] = useState('');
  const [sending, setSending] = useState(false);

  const canMoveStepOne = useMemo(() => formData.name.trim() && formData.role.trim(), [formData.name, formData.role]);

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
      triggerToast(`${type === 'profile' ? 'Profile picture' : 'Banner'} uploaded`);
    } catch (err) {
      triggerToast('Failed to upload image', 'error');
    }
  };

  const handleNext = async () => {
    if (step === 1 && !canMoveStepOne) {
      triggerToast('Please fill the required fields', 'error');
      return;
    }

    if (step === 2) {
      if (formData.platform === 'hermitchat') {
        await handleCreate();
        return;
      }
      await handleSendCode();
      return;
    }

    if (step < 3) setStep(step + 1);
  };

  const handleBack = () => {
    if (step > 1) setStep(step - 1);
  };

  const handleSendCode = async () => {
    if (!telegramData.botToken || !telegramData.allowedUserId) {
      triggerToast('Please fill bot token and user ID', 'error');
      return;
    }
    setSending(true);
    try {
      await fetch(`${API_BASE}/api/telegram/send-code`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          token: telegramData.botToken,
          userId: telegramData.allowedUserId,
        }),
      });
      triggerToast('Verification code sent!');
      setStep(3);
    } catch (err) {
      triggerToast('Failed to send code', 'error');
    } finally {
      setSending(false);
    }
  };

  const handleVerify = async () => {
    if (!verifyCode) {
      triggerToast('Please enter the code', 'error');
      return;
    }
    setSending(true);
    try {
      const res = await fetch(`${API_BASE}/api/telegram/verify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          token: telegramData.botToken,
          code: verifyCode,
          userId: telegramData.allowedUserId,
        }),
      });
      const data = await res.json();
      if (data.success) {
        await handleCreate();
      } else {
        triggerToast(data.error || 'Invalid code', 'error');
      }
    } catch (err) {
      triggerToast('Verification failed', 'error');
    } finally {
      setSending(false);
    }
  };

  const handleCreate = async () => {
    try {
      await fetch(`${API_BASE}/api/agents`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: formData.name,
          role: formData.role,
          personality: formData.personality,
          provider: formData.provider,
          profilePic: formData.profilePic,
          bannerUrl: formData.bannerUrl,
          telegramToken: formData.platform === 'telegram' ? telegramData.botToken : '',
          telegramId: formData.platform === 'telegram' ? telegramData.allowedUserId : '',
          model: formData.model,
          allowedUsers: formData.allowedUsers,
          platform: formData.platform,
          status: 'running',
        }),
      });
      triggerToast('Agent deployed successfully!');
      fetchAgents();
      onClose();
    } catch (err) {
      triggerToast('Failed to create agent', 'error');
    }
  };

  return (
    <div className="fixed inset-0 bg-black/85 backdrop-blur-md flex items-center justify-center z-50 p-4 sm:p-6 animate-in fade-in duration-300">
      <div className="bg-zinc-950 border border-zinc-800 w-full max-w-3xl rounded-[2.5rem] p-6 sm:p-10 relative flex flex-col shadow-2xl max-h-[min(92vh,960px)] overflow-y-auto">
        <button onClick={onClose} className="absolute top-6 right-6 sm:top-8 sm:right-8 w-11 h-11 bg-zinc-900 rounded-full flex items-center justify-center text-zinc-500 hover:text-white transition-all">
          <X className="w-6 h-6" />
        </button>

        <h2 className="text-3xl sm:text-4xl font-black text-white mb-2 lowercase">new deployment</h2>
        <p className="text-zinc-500 text-sm lowercase mb-8 border-b border-zinc-800 pb-6">stage {step} of 3</p>

        <div className="flex-1">
          {step === 1 && (
            <div className="space-y-6">
              <div>
                <label className="block text-sm text-zinc-400 mb-2 ml-4">Agent Name *</label>
                <input type="text" value={formData.name} onChange={e => setFormData({ ...formData, name: e.target.value })} placeholder="e.g. Ralph" className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors" />
              </div>
              <div>
                <label className="block text-sm text-zinc-400 mb-2 ml-4">Role *</label>
                <input type="text" value={formData.role} onChange={e => setFormData({ ...formData, role: e.target.value })} placeholder="e.g. Code Assistant" className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors" />
              </div>
              <div>
                <label className="block text-sm text-zinc-400 mb-2 ml-4">Personality</label>
                <input type="text" value={formData.personality} onChange={e => setFormData({ ...formData, personality: e.target.value })} placeholder="e.g. Helpful and concise" className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors" />
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="bg-black border border-zinc-800 rounded-2xl p-4">
                  <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-2">Profile image URL</label>
                  <input type="text" value={formData.profilePic} onChange={e => setFormData({ ...formData, profilePic: e.target.value })} placeholder="https://..." className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-2.5 text-white outline-none focus:border-zinc-500 transition-colors text-sm" />
                  <label className="mt-3 inline-flex items-center gap-2 cursor-pointer text-xs text-zinc-300 hover:text-white">
                    <UploadCloud className="w-4 h-4" /> Upload profile
                    <input type="file" accept="image/*" className="hidden" onChange={e => e.target.files?.[0] && uploadImage(e.target.files[0], 'profile')} />
                  </label>
                </div>

                <div className="bg-black border border-zinc-800 rounded-2xl p-4">
                  <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-2">Banner image URL</label>
                  <input type="text" value={formData.bannerUrl} onChange={e => setFormData({ ...formData, bannerUrl: e.target.value })} placeholder="https://..." className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-2.5 text-white outline-none focus:border-zinc-500 transition-colors text-sm" />
                  <label className="mt-3 inline-flex items-center gap-2 cursor-pointer text-xs text-zinc-300 hover:text-white">
                    <ImagePlus className="w-4 h-4" /> Upload banner
                    <input type="file" accept="image/*" className="hidden" onChange={e => e.target.files?.[0] && uploadImage(e.target.files[0], 'banner')} />
                  </label>
                </div>
              </div>
            </div>
          )}

          {step === 2 && (
            <div className="space-y-4">
              <label className="block text-sm text-zinc-400 mb-4 ml-4">Select LLM Provider</label>
              {['openrouter', 'openai', 'anthropic', 'gemini'].map(provider => (
                <button
                  key={provider}
                  onClick={() => setFormData({ ...formData, provider })}
                  className={`w-full text-left px-8 py-5 rounded-full border transition-all ${formData.provider === provider ? 'bg-white text-black border-white font-bold' : 'bg-black border-zinc-800 text-white hover:border-zinc-600'}`}
                >
                  {provider === 'openai' ? 'OpenAI' :
                    provider === 'gemini' ? 'Google Gemini' :
                      provider.charAt(0).toUpperCase() + provider.slice(1)}
                </button>
              ))}

              <div className="mt-6 pt-6 border-t border-zinc-800">
                <label className="block text-sm text-zinc-400 mb-4 ml-4">Deployment Platform</label>
                <div className="grid grid-cols-2 gap-4">
                  <button
                    onClick={() => setFormData({ ...formData, platform: 'telegram' })}
                    className={`px-6 py-4 rounded-full border transition-all ${formData.platform === 'telegram' ? 'bg-white text-black border-white font-bold' : 'bg-black border-zinc-800 text-white'}`}
                  >
                    Telegram
                  </button>
                  <button
                    onClick={() => setFormData({ ...formData, platform: 'hermitchat' })}
                    className={`px-6 py-4 rounded-full border transition-all ${formData.platform === 'hermitchat' ? 'bg-white text-black border-white font-bold' : 'bg-black border-zinc-800 text-white'}`}
                  >
                    HermitChat
                  </button>
                </div>
              </div>

              <div className="mt-6 pt-6 border-t border-zinc-800">
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm text-zinc-400 mb-2 ml-4">Model (e.g. gemini-3-flash-preview)</label>
                    <input type="text" value={formData.model} onChange={e => setFormData({ ...formData, model: e.target.value })} placeholder="Enter specific model name" className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors" />
                  </div>

                  {formData.platform === 'telegram' && (
                    <>
                      <div>
                        <label className="block text-sm text-zinc-400 mb-2 ml-4">Telegram Bot Token</label>
                        <input type="text" value={telegramData.botToken} onChange={e => setTelegramData({ ...telegramData, botToken: e.target.value })} placeholder="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11" className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors" />
                      </div>
                      <div>
                        <label className="block text-sm text-zinc-400 mb-2 ml-4">Primary User (from Allowlist)</label>
                        <select
                          value={telegramData.allowedUserId}
                          onChange={e => {
                            const selected = allowlist.find(a => a.telegramUserId === e.target.value);
                            setTelegramData({ ...telegramData, allowedUserId: e.target.value });
                            if (selected) {
                              setFormData({ ...formData, allowedUsers: e.target.value });
                            } else {
                              setFormData({ ...formData, allowedUsers: '' });
                            }
                          }}
                          className="w-full bg-black border border-zinc-800 rounded-full px-8 py-4 text-white outline-none focus:border-zinc-500 transition-colors appearance-none"
                        >
                          <option value="">Select a user</option>
                          {allowlist.map(item => (
                            <option key={item.telegramUserId} value={item.telegramUserId}>{item.friendlyName} ({item.telegramUserId})</option>
                          ))}
                        </select>
                      </div>
                      <p className="text-xs text-zinc-500 ml-4">A 6-digit code will be sent to verify the bot and associate it with the selected primary user.</p>
                    </>
                  )}

                  {formData.platform === 'hermitchat' && (
                    <p className="text-xs text-zinc-500 ml-4 bg-zinc-900 p-4 rounded-2xl border border-zinc-800">
                      HermitChat deployment doesn't require verification. The agent will be immediately available in your mobile app.
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          {step === 3 && (
            <div className="space-y-6 text-center flex flex-col items-center">
              <p className="text-zinc-400 mb-4">Enter the 6-digit code sent to your Telegram.</p>
              <input type="text" value={verifyCode} onChange={e => setVerifyCode(e.target.value)} placeholder="000000" className="w-48 bg-black border border-zinc-800 rounded-2xl px-6 py-4 text-white text-center text-2xl tracking-[0.5em] outline-none focus:border-zinc-500 transition-colors font-mono" maxLength={6} />
              <button className="text-sm text-zinc-500 hover:text-white underline mt-4" onClick={handleSendCode}>Resend Code</button>
            </div>
          )}
        </div>

        <div className="flex justify-between pt-10 mt-auto">
          {step > 1 ? (
            <button onClick={handleBack} className="text-zinc-500 hover:text-white px-6 py-4 font-bold">back</button>
          ) : <div></div>}

          {step < 3 ? (
            <button onClick={handleNext} disabled={sending} className="bg-white text-black px-10 py-4 rounded-full font-bold hover:bg-zinc-200 transition-colors disabled:opacity-60">
              {step === 2
                ? (formData.platform === 'hermitchat' ? 'deploy agent' : (sending ? 'sending code...' : 'send code'))
                : 'next stage'}
            </button>
          ) : (
            <button onClick={handleVerify} disabled={sending} className="bg-emerald-500 text-black px-10 py-4 rounded-full font-bold hover:bg-emerald-400 transition-colors disabled:opacity-50">
              {sending ? 'Verifying...' : 'verify & deploy'}
            </button>
          )}
        </div>
      </div>
    </div >
  );
}
