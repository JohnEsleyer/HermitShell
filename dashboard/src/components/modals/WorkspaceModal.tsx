import { useState, useEffect } from 'react';
import { X, Folder, File, Download, Trash2, HardDrive } from 'lucide-react';
import { ContainerItem } from '../../types';

interface WorkspaceModalProps {
  container: ContainerItem;
  onClose: () => void;
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
}

interface FileItem {
  name: string;
  size: number;
  mode: string;
  modTime: string;
  isDir: boolean;
}

interface FilesResponse {
  path: string;
  files: FileItem[];
}

const API_BASE = '';

function formatSize(bytes: number): string {
  if (bytes === 0) return '--';
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

export function WorkspaceModal({ container, onClose, triggerToast }: WorkspaceModalProps) {
  const [currentFolder, setCurrentFolder] = useState('/app/workspace');
  const [files, setFiles] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);

  const folders = [
    { path: '/app/workspace/work', name: 'work' },
    { path: '/app/workspace/in', name: 'in' },
    { path: '/app/workspace/out', name: 'out' },
    { path: '/app/workspace/apps', name: 'apps' },
  ];

  useEffect(() => {
    fetchFiles(currentFolder);
  }, [currentFolder, container.id]);

  const fetchFiles = async (path: string) => {
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/containers/${container.id}/files?path=${encodeURIComponent(path)}`);
      const data: FilesResponse = await res.json();
      setFiles(data.files || []);
    } catch (err) {
      console.error('Failed to fetch files:', err);
      setFiles([]);
    } finally {
      setLoading(false);
    }
  };

  const handleFolderClick = (path: string) => {
    setCurrentFolder(path);
  };

  return (
    <div className="fixed inset-0 bg-black/90 backdrop-blur-md flex items-center justify-center z-50 p-6 animate-in fade-in duration-300">
      <div className="bg-zinc-950 border border-zinc-800 w-full max-w-5xl h-[80vh] rounded-[2.5rem] relative flex flex-col shadow-2xl overflow-hidden">
        
        <div className="p-6 border-b border-zinc-800 flex justify-between items-center bg-zinc-900/50">
          <div>
            <h2 className="text-2xl font-bold text-white flex items-center gap-3">
              <HardDrive className="w-6 h-6" /> Workspace: {container.agentName}
            </h2>
            <p className="text-sm text-zinc-400 mt-1 font-mono">{container.id}</p>
          </div>
          <button onClick={onClose} className="w-10 h-10 bg-zinc-800 rounded-full flex items-center justify-center text-zinc-400 hover:text-white transition-all">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="flex flex-1 overflow-hidden">
          <div className="w-64 border-r border-zinc-800 bg-[#0a0a0a] p-4 flex flex-col gap-2">
            <div className="text-xs font-bold text-zinc-500 uppercase tracking-wider mb-2 px-2">Directories</div>
            {folders.map(folder => (
              <button 
                key={folder.path}
                onClick={() => handleFolderClick(folder.path)}
                className={`flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-mono transition-all text-left ${currentFolder === folder.path ? 'bg-zinc-900 text-white' : 'text-zinc-400 hover:bg-zinc-900/50 hover:text-white'}`}
              >
                <Folder className="w-4 h-4 shrink-0" />
                <span className="truncate">{folder.name}/</span>
              </button>
            ))}
          </div>

          <div className="flex-1 flex flex-col bg-zinc-950">
            <div className="p-4 border-b border-zinc-800 bg-zinc-900/30 font-mono text-sm text-zinc-400 flex items-center gap-2">
              <Folder className="w-4 h-4" /> {currentFolder}/
            </div>
            
            <div className="flex-1 overflow-y-auto p-4">
              {loading ? (
                <div className="h-full flex flex-col items-center justify-center text-zinc-500">
                  <div className="w-8 h-8 border-2 border-zinc-800 border-t-white rounded-full animate-spin"></div>
                  <p className="mt-4">Loading files...</p>
                </div>
              ) : files.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-zinc-500">
                  <Folder className="w-12 h-12 mb-4 opacity-20" />
                  <p>Directory is empty</p>
                </div>
              ) : (
                <div className="grid grid-cols-1 gap-2">
                  {files.map(file => (
                    <div key={file.name} className="flex items-center justify-between p-4 rounded-xl hover:bg-zinc-900/50 border border-transparent hover:border-zinc-800 transition-all group">
                      <div className="flex items-center gap-4">
                        {file.isDir ? <Folder className="w-5 h-5 text-blue-400" /> : <File className="w-5 h-5 text-zinc-400" />}
                        <div>
                          <div className="font-mono text-sm text-white">{file.name}</div>
                          <div className="text-xs text-zinc-500 mt-1">{formatSize(file.size)} • {file.modTime}</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                        {file.isDir && (
                          <button 
                            onClick={() => setCurrentFolder(currentFolder + '/' + file.name)}
                            className="p-2 text-zinc-400 hover:text-white hover:bg-zinc-800 rounded-lg transition-colors"
                          >
                            <Folder className="w-4 h-4" />
                          </button>
                        )}
                        {!file.isDir && (
                          <button onClick={() => triggerToast(`Downloading ${file.name}...`)} className="p-2 text-zinc-400 hover:text-white hover:bg-zinc-800 rounded-lg transition-colors">
                            <Download className="w-4 h-4" />
                          </button>
                        )}
                        <button onClick={() => triggerToast(`Deleted ${file.name}`)} className="p-2 text-red-400 hover:text-red-300 hover:bg-red-950/50 rounded-lg transition-colors">
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
