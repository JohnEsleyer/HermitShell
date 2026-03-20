import { useState, useEffect } from 'react';
import { Globe, Key, RefreshCw, LogOut, User, Download, Upload, Archive, AlertTriangle } from 'lucide-react';

const API_BASE = '';

interface SettingsTabProps {
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
  onLogout: () => void;
}

export function SettingsTab({ triggerToast, onLogout }: SettingsTabProps) {
  const [loading, setLoading] = useState(true);
  const [settings, setSettings] = useState({
    tunnelEnabled: true,
    tunnelURL: '',
    tunnelHealthy: false,
    timezone: 'UTC',
    timeOffset: '0',
    hasLLMKey: false,
    currentTime: '',
    currentTime12: '',
    currentDate: '',
    serverUtcTime: '',
  });
  const [saving, setSaving] = useState(false);

  // Real local time (from browser)
  const [localTime, setLocalTime] = useState('');

  // Update local time every second
  useEffect(() => {
    const updateLocalTime = () => {
      const now = new Date();
      setLocalTime(now.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', second: '2-digit', hour12: true }));
    };
    updateLocalTime();
    const interval = setInterval(updateLocalTime, 1000);
    return () => clearInterval(interval);
  }, []);

  // Calculate preview time based on selected offset.
  // Docs: See docs/time-management.md for preview calculation logic.
  // How it works: Converts local browser time to UTC, then adds offset to preview the result.
  const getPreviewTime = () => {
    // We use serverUtcTime (actual UTC) as the base for all calculations to avoid local clock drift
    const base = settings.serverUtcTime ? new Date(settings.serverUtcTime) : new Date();
    const offset = parseInt(settings.timeOffset || '0');
    // serverUtcTime is already UTC, so we just add the offset hours
    const preview = new Date(base.getTime() + (offset * 3600000));
    return preview.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', second: '2-digit', hour12: true });
  };
  const [apiKeys, setApiKeys] = useState({
    openrouterKey: '',
    openaiKey: '',
    anthropicKey: '',
    geminiKey: '',
  });

  const [hasApiKeys, setHasApiKeys] = useState({
    openrouterKey: false,
    openaiKey: false,
    anthropicKey: false,
    geminiKey: false,
  });

  const [credentials, setCredentials] = useState({
    newUsername: '',
    newPassword: '',
  });

  // Backup/Restore state
  const [importPassword, setImportPassword] = useState('');
  const [importing, setImporting] = useState(false);
  const [exporting, setExporting] = useState(false);

  // Handle Export - downloads all app data as a zip file
  const handleExport = async () => {
    setExporting(true);
    try {
      const response = await fetch(`${API_BASE}/api/backup/export`, {
        method: 'GET',
        credentials: 'include',
      });

      if (!response.ok) {
        throw new Error('Export failed');
      }

      // Get the blob and download it
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;

      // Extract filename from content-disposition header or generate one
      const contentDisposition = response.headers.get('Content-Disposition');
      let filename = `hermit-backup-${new Date().toISOString().slice(0, 10)}.zip`;
      if (contentDisposition) {
        const match = contentDisposition.match(/filename="(.+)"/);
        if (match) filename = match[1];
      }

      a.download = filename;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      a.remove();

      triggerToast('Backup exported successfully');
    } catch (err) {
      console.error('Export error:', err);
      triggerToast('Failed to export backup', 'error');
    } finally {
      setExporting(false);
    }
  };

  // Handle Import - uploads a backup zip file
  const handleImport = async (e: React.FormEvent) => {
    e.preventDefault();

    const fileInput = document.getElementById('backup-file') as HTMLInputElement;
    const file = fileInput?.files?.[0];

    if (!file) {
      triggerToast('Please select a backup file', 'error');
      return;
    }

    if (!importPassword) {
      triggerToast('Please enter your password', 'error');
      return;
    }

    setImporting(true);
    try {
      const formData = new FormData();
      formData.append('backup', file);
      formData.append('password', importPassword);

      const response = await fetch(`${API_BASE}/api/backup/import`, {
        method: 'POST',
        body: formData,
        credentials: 'include',
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || 'Import failed');
      }

      triggerToast('Backup imported successfully. Some changes may require restart.');
      setImportPassword('');
      if (fileInput) fileInput.value = '';
    } catch (err: any) {
      console.error('Import error:', err);
      triggerToast(err.message || 'Failed to import backup', 'error');
    } finally {
      setImporting(false);
    }
  };

  // Fetch settings and current time from backend.
  // Docs: See docs/time-management.md for time settings persistence.
  // How it works: Loads time_offset and timezone from API, applies to preview.
  const fetchSettings = async () => {
    setLoading(true);
    try {
      const [settingsRes, timeRes] = await Promise.all([
        fetch(`${API_BASE}/api/settings`),
        fetch(`${API_BASE}/api/time`),
      ]);
      const data = await settingsRes.json();
      const timeData = await timeRes.json();

      setSettings({
        ...data,
        timezone: data.timezone || 'UTC',
        timeOffset: data.timeOffset || '0',
        currentTime: timeData.time || '',
        currentTime12: timeData.time12 || '',
        currentDate: timeData.date || '',
        serverUtcTime: timeData.serverUtcTime || '',
      });
      setHasApiKeys({
        openrouterKey: !!data.openrouterKey,
        openaiKey: !!data.openaiKey,
        anthropicKey: !!data.anthropicKey,
        geminiKey: !!data.geminiKey,
      });
      setApiKeys({
        openrouterKey: '',
        openaiKey: '',
        anthropicKey: '',
        geminiKey: '',
      });
    } catch (err) {
      console.error('Failed to fetch settings:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSettings();
  }, []);

  const handleSave = async (specificKeys?: any) => {
    setSaving(true);
    try {
      const payload: any = {
        tunnelEnabled: settings.tunnelEnabled,
        timezone: settings.timezone,
        timeOffset: settings.timeOffset,
        ...specificKeys
      };

      if (!specificKeys) {
        if (apiKeys.openrouterKey) payload.openrouterKey = apiKeys.openrouterKey;
        if (apiKeys.openaiKey) payload.openaiKey = apiKeys.openaiKey;
        if (apiKeys.anthropicKey) payload.anthropicKey = apiKeys.anthropicKey;
        if (apiKeys.geminiKey) payload.geminiKey = apiKeys.geminiKey;
      }

      const res = await fetch(`${API_BASE}/api/settings`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        triggerToast('Settings saved successfully');
        await fetchSettings();
      } else {
        triggerToast('Failed to save settings', 'error');
      }
    } catch (err) {
      triggerToast('Failed to save settings', 'error');
    } finally {
      setSaving(false);
    }
  };

  const handleClearKey = (key: string) => {
    if (confirm(`Are you sure you want to clear the ${key}?`)) {
      handleSave({ [key]: "REMOVE" });
    }
  };

  const handleSaveCredentials = async () => {
    if (!credentials.newUsername || !credentials.newPassword) {
      triggerToast('Username and password are required', 'error');
      return;
    }
    setSaving(true);
    try {
      const res = await fetch(`${API_BASE}/api/auth/change-credentials`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(credentials),
      });
      const data = await res.json();
      if (data.success) {
        triggerToast('Credentials updated successfully');
        setCredentials({ newUsername: '', newPassword: '' });
      } else {
        triggerToast(data.error || 'Failed to update credentials', 'error');
      }
    } catch (err) {
      triggerToast('Failed to update credentials', 'error');
    } finally {
      setSaving(false);
    }
  };

  const refreshTunnel = async () => {
    triggerToast('Refreshing tunnel...', 'info');
  };

  return (
    <div className="flex-1 flex flex-col gap-8 max-w-4xl">
      <div className="bg-black border border-zinc-800 rounded-[2.5rem] p-8">
        <h2 className="text-2xl font-bold mb-6 flex items-center gap-3"><Globe className="w-6 h-6" /> Public URL Configuration</h2>

        <div className="flex justify-between items-center mb-8">
          <div>
            <h3 className="font-bold">Enable Cloudflare Tunnel</h3>
            <p className="text-sm text-zinc-400">Allows external access to the dashboard and apps.</p>
          </div>
          <button
            onClick={() => {
              const newValue = !settings.tunnelEnabled;
              setSettings({ ...settings, tunnelEnabled: newValue });
              handleSave({ tunnelEnabled: newValue });
            }}
            className={`w-14 h-8 rounded-full transition-colors relative ${settings.tunnelEnabled ? 'bg-emerald-500' : 'bg-zinc-700'}`}
          >
            <div className={`absolute top-1 left-1 bg-white w-6 h-6 rounded-full transition-transform ${settings.tunnelEnabled ? 'translate-x-6' : 'translate-x-0'}`}></div>
          </button>
        </div>

        {settings.tunnelEnabled && (
          <div className="space-y-4 animate-in fade-in">
            <p className="text-sm text-zinc-400">The system automatically orchestrates cloudflared CLI to create a tunnel URL for the dashboard and agents.</p>
            <div className="bg-zinc-950 border border-zinc-800 rounded-2xl p-4 flex items-center justify-between">
              <div>
                <div className="text-xs text-zinc-500 uppercase tracking-wider mb-1">Current Tunnel URL</div>
                <div className="font-mono text-emerald-400">{settings.tunnelURL || 'No tunnel active'}</div>
              </div>
              <button onClick={refreshTunnel} className="bg-zinc-900 hover:bg-zinc-800 text-white px-4 py-2 rounded-xl text-sm font-bold transition-colors flex items-center gap-2">
                <RefreshCw className="w-4 h-4" /> Refresh Tunnel
              </button>
            </div>
            <div className="flex items-center gap-2 text-xs text-zinc-500 mt-2">
              <div className={`w-2 h-2 rounded-full ${settings.tunnelHealthy ? 'bg-emerald-500 animate-pulse' : 'bg-zinc-500'}`}></div>
              Tunnel health check: {settings.tunnelHealthy ? 'OK' : 'Disconnected'}
            </div>
          </div>
        )}
      </div>

      <div className="bg-black border border-zinc-800 rounded-[2.5rem] p-8">
        <h2 className="text-2xl font-bold mb-6 flex items-center gap-3"><Key className="w-6 h-6" /> API Keys</h2>
        <div className="space-y-4">
          <div>
            <div className="flex justify-between items-center mb-2">
              <label className="block text-xs text-zinc-500 uppercase tracking-wider">OpenRouter API Key (Free Models)</label>
              {hasApiKeys.openrouterKey && <button onClick={() => handleClearKey('openrouterKey')} className="text-[10px] text-red-500 hover:text-red-400 font-bold uppercase">Clear Key</button>}
            </div>
            <input
              type="password"
              value={apiKeys.openrouterKey}
              onChange={e => setApiKeys({ ...apiKeys, openrouterKey: e.target.value })}
              placeholder={hasApiKeys.openrouterKey ? "••••••••••••••••" : "sk-or-..."}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-3 text-white outline-none focus:border-zinc-600"
            />
          </div>
          <div>
            <div className="flex justify-between items-center mb-2">
              <label className="block text-xs text-zinc-500 uppercase tracking-wider">OpenAI API Key</label>
              {hasApiKeys.openaiKey && <button onClick={() => handleClearKey('openaiKey')} className="text-[10px] text-red-500 hover:text-red-400 font-bold uppercase">Clear Key</button>}
            </div>
            <input
              type="password"
              value={apiKeys.openaiKey}
              onChange={e => setApiKeys({ ...apiKeys, openaiKey: e.target.value })}
              placeholder={hasApiKeys.openaiKey ? "••••••••••••••••" : "sk-..."}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-3 text-white outline-none focus:border-zinc-600"
            />
          </div>
          <div>
            <div className="flex justify-between items-center mb-2">
              <label className="block text-xs text-zinc-500 uppercase tracking-wider">Anthropic API Key</label>
              {hasApiKeys.anthropicKey && <button onClick={() => handleClearKey('anthropicKey')} className="text-[10px] text-red-500 hover:text-red-400 font-bold uppercase">Clear Key</button>}
            </div>
            <input
              type="password"
              value={apiKeys.anthropicKey}
              onChange={e => setApiKeys({ ...apiKeys, anthropicKey: e.target.value })}
              placeholder={hasApiKeys.anthropicKey ? "••••••••••••••••" : "sk-ant-..."}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-3 text-white outline-none focus:border-zinc-600"
            />
          </div>
          <div>
            <div className="flex justify-between items-center mb-2">
              <label className="block text-xs text-zinc-500 uppercase tracking-wider">Gemini API Key</label>
              {hasApiKeys.geminiKey && <button onClick={() => handleClearKey('geminiKey')} className="text-[10px] text-red-500 hover:text-red-400 font-bold uppercase">Clear Key</button>}
            </div>
            <input
              type="password"
              value={apiKeys.geminiKey}
              onChange={e => setApiKeys({ ...apiKeys, geminiKey: e.target.value })}
              placeholder={hasApiKeys.geminiKey ? "••••••••••••••••" : "AIza..."}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-3 text-white outline-none focus:border-zinc-600"
            />
          </div>
          <div className="flex justify-end mt-4">
            <button
              onClick={() => handleSave()}
              disabled={saving}
              className="bg-white text-black px-6 py-3 rounded-xl text-sm font-bold hover:bg-zinc-200 transition-colors disabled:opacity-50"
            >
              {saving ? 'Saving...' : 'Save Keys'}
            </button>
          </div>
        </div>
      </div>

      <div className="bg-black border border-zinc-800 rounded-[2.5rem] p-8">
        <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">System Time</h2>

        {loading ? (
          <div className="space-y-6">
            <div className="bg-zinc-900/50 rounded-2xl p-6 border border-zinc-800">
              <div className="animate-pulse space-y-4">
                <div className="h-8 bg-zinc-800 rounded w-32"></div>
                <div className="h-12 bg-zinc-800 rounded w-48"></div>
              </div>
            </div>
          </div>
        ) : (
          <div className="space-y-6">
            {/* Time Display Cards */}
            <div className="grid grid-cols-3 gap-4">
              {/* Active System Time (from Server) */}
              <div className="bg-emerald-500/10 rounded-2xl p-6 border border-emerald-500/30 shadow-[0_0_20px_rgba(16,185,129,0.1)]">
                <div className="text-xs text-emerald-400 uppercase tracking-wider mb-1 font-black">Global System Time</div>
                <div className="text-3xl font-mono font-bold text-emerald-400">
                  {settings.currentTime12 || '--:--:--'}
                </div>
                <div className="text-[10px] text-emerald-400/70 mt-1 flex items-center gap-1.5 font-bold">
                  <Globe className="w-3 h-3" />
                  ACTIVE OFFSET: UTC{parseInt(settings.timeOffset || '0') >= 0 ? '+' : ''}{settings.timeOffset || 0}h
                </div>
              </div>

              {/* Your Browser Time */}
              <div className="bg-zinc-900/50 rounded-2xl p-6 border border-zinc-800">
                <div className="text-xs text-zinc-500 uppercase tracking-wider mb-1">Your Browser Time</div>
                <div className="text-3xl font-mono font-bold text-zinc-400">
                  {localTime}
                </div>
                <div className="text-[10px] text-zinc-500/70 mt-1 font-bold">
                  FROM COMPUTER
                </div>
              </div>

              {/* System Time Preview (based on selection) */}
              <div className="bg-blue-500/10 rounded-2xl p-6 border border-blue-500/30">
                <div className="text-xs text-blue-400 uppercase tracking-wider mb-1">System Time Preview</div>
                <div className="text-3xl font-mono font-bold text-blue-400">
                  {getPreviewTime()}
                </div>
                <div className="text-[10px] text-blue-400/70 mt-1 font-bold">
                  SELECTED: UTC{parseInt(settings.timeOffset || '0') >= 0 ? '+' : ''}{settings.timeOffset || 0}h
                </div>
              </div>
            </div>

            {/* Custom Offset Slider */}
            <div className="bg-zinc-900/30 border border-zinc-800 rounded-2xl p-6 space-y-4">
              <div className="flex justify-between items-center">
                <label className="block text-xs text-zinc-500 uppercase tracking-wider font-bold">Custom Slider</label>
                <div className="px-3 py-1 bg-blue-500 text-white rounded-lg font-black text-sm shadow-lg shadow-blue-500/20">
                  {settings.timeOffset}h
                </div>
              </div>
              <input
                type="range"
                min="-12"
                max="14"
                step="1"
                value={settings.timeOffset}
                onChange={e => setSettings({ ...settings, timeOffset: e.target.value })}
                className="w-full h-2 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-blue-500 hover:accent-blue-400 transition-all border border-zinc-700"
              />
              <div className="flex justify-between text-[10px] text-zinc-500 font-medium px-1">
                <span>-12h</span>
                <span>UTC</span>
                <span>+14h</span>
              </div>
            </div>

            {/* Offset Presets */}
            <div>
              <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-3">
                Presets
              </label>
              <div className="grid grid-cols-4 gap-3">
                {[
                  { label: 'UTC', value: '0', desc: 'London' },
                  { label: '+8h', value: '8', desc: 'Philippines' },
                  { label: '+9h', value: '9', desc: 'Tokyo' },
                  { label: '+1h', value: '1', desc: 'Paris' },
                  { label: '-5h', value: '-5', desc: 'New York' },
                  { label: '-8h', value: '-8', desc: 'Los Angeles' },
                  { label: '+5h', value: '5', desc: 'Dubai' },
                  { label: '+3h', value: '3', desc: 'Moscow' },
                ].map(preset => (
                  <button
                    key={preset.value}
                    type="button"
                    onClick={() => {
                      setSettings(s => ({ ...s, timeOffset: preset.value }));
                    }}
                    className={`py-3 px-4 rounded-xl text-sm font-medium transition-all ${settings.timeOffset === preset.value
                      ? 'bg-blue-500 text-white'
                      : 'bg-zinc-800 text-zinc-400 hover:text-white hover:bg-zinc-700'
                      }`}
                  >
                    <div className="font-bold">{preset.label}</div>
                    <div className="text-xs opacity-70">{preset.desc}</div>
                  </button>
                ))}
              </div>
            </div>

            <div className="flex justify-between items-center pt-2">
              <p className="text-xs text-zinc-500">
                This offset is applied to all scheduled events and the dashboard clock.
              </p>
              <button
                onClick={async () => {
                  setSaving(true);
                  try {
                    const res = await fetch(`${API_BASE}/api/settings`, {
                      method: 'POST',
                      headers: { 'Content-Type': 'application/json' },
                      body: JSON.stringify({
                        timeOffset: settings.timeOffset,
                        timezone: settings.timezone,
                      }),
                    });
                    if (res.ok) {
                      triggerToast('Time settings saved');
                      const timeRes = await fetch(`${API_BASE}/api/time`);
                      const timeData = await timeRes.json();
                      setSettings(s => ({
                        ...s,
                        currentTime: timeData.time,
                        currentTime12: timeData.time12,
                        currentDate: timeData.date,
                        serverUtcTime: timeData.serverUtcTime,
                        timeOffset: timeData.offset || timeData.timeOffset || s.timeOffset
                      }));
                    } else {
                      triggerToast('Failed to save', 'error');
                    }
                  } catch (err) {
                    triggerToast('Failed to save', 'error');
                  }
                  setSaving(false);
                }}
                disabled={saving}
                className="bg-white text-black px-8 py-3 rounded-full text-sm font-bold hover:bg-zinc-200 transition-colors disabled:opacity-50"
              >
                {saving ? 'Saving...' : 'Save Time Settings'}
              </button>
            </div>
          </div>
        )}
      </div>

      <div className="bg-black border border-zinc-800 rounded-[2.5rem] p-8">
        <h2 className="text-2xl font-bold mb-6 flex items-center gap-3"><User className="w-6 h-6" /> Account Credentials</h2>
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">New Username</label>
            <input
              type="text"
              value={credentials.newUsername}
              onChange={e => setCredentials({ ...credentials, newUsername: e.target.value })}
              placeholder="Enter new username"
              className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-3 text-white outline-none focus:border-zinc-600"
            />
          </div>
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">New Password</label>
            <input
              type="password"
              value={credentials.newPassword}
              onChange={e => setCredentials({ ...credentials, newPassword: e.target.value })}
              placeholder="Enter new password"
              className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-3 text-white outline-none focus:border-zinc-600"
            />
          </div>
          <div className="flex justify-end mt-4">
            <button
              onClick={handleSaveCredentials}
              disabled={saving}
              className="bg-white text-black px-6 py-3 rounded-xl text-sm font-bold hover:bg-zinc-200 transition-colors disabled:opacity-50"
            >
              {saving ? 'Saving...' : 'Update Credentials'}
            </button>
          </div>
        </div>
      </div>

      <div className="bg-black border border-zinc-800 rounded-[2.5rem] p-8">
        <h2 className="text-2xl font-bold mb-6 flex items-center gap-3"><LogOut className="w-6 h-6" /> Session</h2>
        <button
          onClick={onLogout}
          className="bg-red-950 hover:bg-red-900 text-red-400 px-6 py-3 rounded-xl text-sm font-bold transition-colors flex items-center gap-2"
        >
          <LogOut className="w-4 h-4" /> Logout
        </button>
      </div>

      {/* Backup and Restore Section */}
      {/* Docs: See docs/backup-restore.md for backup and restore documentation */}
      <div className="bg-black border border-zinc-800 rounded-[2.5rem] p-8">
        <h2 className="text-2xl font-bold mb-6 flex items-center gap-3"><Archive className="w-6 h-6" /> Backup & Restore</h2>

        <div className="space-y-8">
          {/* Export Section */}
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <Download className="w-5 h-5 text-emerald-400" />
              <h3 className="text-lg font-semibold">Export Backup</h3>
            </div>
            <p className="text-sm text-zinc-400">
              Download all your data including database, images, skills, agent configurations, and logs as a .zip file.
              Use this to move your data to a new VPS or create a backup.
            </p>
            <button
              onClick={handleExport}
              disabled={exporting}
              className="bg-emerald-600 hover:bg-emerald-500 text-white px-6 py-3 rounded-xl text-sm font-bold transition-colors flex items-center gap-2 disabled:opacity-50"
            >
              <Download className="w-4 h-4" />
              {exporting ? 'Exporting...' : 'Download Backup'}
            </button>
          </div>

          {/* Import Section */}
          <div className="border-t border-zinc-800 pt-8 space-y-4">
            <div className="flex items-center gap-3">
              <Upload className="w-5 h-5 text-amber-400" />
              <h3 className="text-lg font-semibold">Import Backup</h3>
            </div>

            <div className="bg-amber-950/30 border border-amber-900/50 rounded-xl p-4 flex items-start gap-3">
              <AlertTriangle className="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" />
              <div className="text-sm text-amber-200">
                <strong>Warning:</strong> Importing a backup will overwrite existing data.
                This action cannot be undone. Make sure to export your current data first if needed.
              </div>
            </div>

            <form onSubmit={handleImport} className="space-y-4">
              <div>
                <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">
                  Select Backup File (.zip)
                </label>
                <input
                  type="file"
                  id="backup-file"
                  accept=".zip"
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-3 text-white outline-none focus:border-zinc-600 file:mr-4 file:py-2 file:px-4 file:rounded-xl file:border-0 file:text-sm file:font-semibold file:bg-zinc-800 file:text-zinc-300 hover:file:bg-zinc-700"
                />
              </div>

              <div>
                <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">
                  Your Password (required for security)
                </label>
                <input
                  type="password"
                  value={importPassword}
                  onChange={e => setImportPassword(e.target.value)}
                  placeholder="Enter your password"
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-3 text-white outline-none focus:border-zinc-600"
                />
              </div>

              <button
                type="submit"
                disabled={importing}
                className="bg-amber-600 hover:bg-amber-500 text-white px-6 py-3 rounded-xl text-sm font-bold transition-colors flex items-center gap-2 disabled:opacity-50"
              >
                <Upload className="w-4 h-4" />
                {importing ? 'Importing...' : 'Import Backup'}
              </button>
            </form>
          </div>
        </div>
      </div>
    </div >
  );
}
