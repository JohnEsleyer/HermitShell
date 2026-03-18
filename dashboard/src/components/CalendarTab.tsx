import { useState, useEffect, useMemo } from 'react';
import { Calendar as CalendarIcon, Plus, Trash2, Clock, Bell, ChevronLeft, ChevronRight, X } from 'lucide-react';
import { CalendarEvent, Agent } from '../types';

const API_BASE = '';

function formatTime12(time24: string): string {
  if (!time24) return '';
  const [hours, minutes] = time24.split(':');
  const h = parseInt(hours || '0', 10);
  const ampm = h >= 12 ? 'PM' : 'AM';
  const h12 = h % 12 || 12;
  return `${h12}:${minutes} ${ampm}`;
}

interface CalendarTabProps {
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
  agents: Agent[];
}

interface CalendarEventWithAgent extends CalendarEvent {
  agentId: number;
  agentName?: string;
  agentPic?: string;
}

function ProfilePic({ src, name, size = 'md' }: { src?: string; name?: string; size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-5 h-5 text-[8px]',
    md: 'w-8 h-8 text-xs',
    lg: 'w-10 h-10 text-sm',
  };

  const initial = name ? name.charAt(0).toUpperCase() : '?';

  if (src) {
    return (
      <img
        src={src}
        alt={name || 'Agent'}
        className={`${sizeClasses[size]} rounded-lg object-cover bg-zinc-700`}
      />
    );
  }

  return (
    <div className={`${sizeClasses[size]} rounded-lg bg-gradient-to-br from-zinc-600 to-zinc-800 flex items-center justify-center font-bold text-zinc-300`}>
      {initial}
    </div>
  );
}

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

function getDaysInMonth(year: number, month: number): (Date | null)[] {
  const firstDay = new Date(year, month, 1);
  const lastDay = new Date(year, month + 1, 0);
  const daysInMonth = lastDay.getDate();
  const startingDay = firstDay.getDay();

  const days: (Date | null)[] = [];

  for (let i = 0; i < startingDay; i++) {
    days.push(null);
  }

  for (let i = 1; i <= daysInMonth; i++) {
    days.push(new Date(year, month, i));
  }

  const remaining = 42 - days.length;
  for (let i = 0; i < remaining; i++) {
    days.push(null);
  }

  return days;
}

function isSameDay(date1: Date, date2: Date): boolean {
  return date1.getFullYear() === date2.getFullYear() &&
    date1.getMonth() === date2.getMonth() &&
    date1.getDate() === date2.getDate();
}

function isToday(date: Date): boolean {
  return isSameDay(date, new Date());
}

function formatDateDisplay(dateStr: string): string {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' });
}

export function CalendarTab({ triggerToast, agents }: CalendarTabProps) {
  const [events, setEvents] = useState<CalendarEventWithAgent[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [showEventDetails, setShowEventDetails] = useState<CalendarEventWithAgent | null>(null);
  const [showDateEvents, setShowDateEvents] = useState<Date | null>(null);
  const [selectedDateForNew, setSelectedDateForNew] = useState<string>('');
  const [newEvent, setNewEvent] = useState({ agentId: 0, date: '', time: '09:00', prompt: '' });
  const [editEvent, setEditEvent] = useState<CalendarEventWithAgent | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentDate, setCurrentDate] = useState(new Date());

  const currentYear = currentDate.getFullYear();
  const currentMonth = currentDate.getMonth();

  const agentMap = useMemo(() => {
    const map: { [key: number]: { name: string; pic: string } } = {};
    agents.forEach(a => { map[a.id] = { name: a.name, pic: a.profilePic || '' }; });
    return map;
  }, [agents]);

  const fetchEvents = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/calendar`);
      if (!res.ok) {
        console.error('Failed to fetch calendar:', res.status);
        setEvents([]);
        return;
      }
      const data = await res.json();
      const eventsWithAgents = (data || []).map((e: CalendarEventWithAgent) => {
        const agentInfo = agentMap[e.agentId] || { name: 'Unknown Agent', pic: '' };
        return {
          ...e,
          agentName: agentInfo.name,
          agentPic: e.agentPic || agentInfo.pic
        };
      });
      setEvents(eventsWithAgents);
    } catch (err) {
      console.error('Failed to fetch calendar:', err);
      setEvents([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchEvents();
    const interval = setInterval(fetchEvents, 10000);
    return () => clearInterval(interval);
  }, [agentMap]);

  const handleCreate = async () => {
    if (!newEvent.agentId || !newEvent.date || !newEvent.time || !newEvent.prompt) {
      triggerToast('Please fill all fields', 'error');
      return;
    }
    try {
      await fetch(`${API_BASE}/api/calendar`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newEvent),
      });
      triggerToast('Event created');
      setShowCreate(false);
      setNewEvent({ agentId: 0, date: '', time: '09:00', prompt: '' });
      fetchEvents();
    } catch (err) {
      triggerToast('Failed to create event', 'error');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await fetch(`${API_BASE}/api/calendar/${id}`, { method: 'DELETE' });
      triggerToast('Event deleted');
      setShowEventDetails(null);
      fetchEvents();
    } catch (err) {
      triggerToast('Failed to delete event', 'error');
    }
  };

  const handleUpdate = async (id: number, agentId: number, date: string, time: string, prompt: string) => {
    try {
      await fetch(`${API_BASE}/api/calendar/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agentId, date, time, prompt }),
      });
      triggerToast('Event updated');
      setShowEventDetails(null);
      fetchEvents();
    } catch (err) {
      triggerToast('Failed to update event', 'error');
    }
  };

  const getEventsForDate = (date: Date): CalendarEvent[] => {
    const dateStr = date.toLocaleDateString('en-CA'); // YYYY-MM-DD in local time
    return events.filter(e => e.date === dateStr);
  };

  const days = useMemo(() => getDaysInMonth(currentYear, currentMonth), [currentYear, currentMonth]);

  const goToPrevMonth = () => {
    setCurrentDate(new Date(currentYear, currentMonth - 1, 1));
  };

  const goToNextMonth = () => {
    setCurrentDate(new Date(currentYear, currentMonth + 1, 1));
  };

  const goToToday = () => {
    setCurrentDate(new Date());
  };

  const openCreateForDate = (date: Date) => {
    const dateStr = date.toLocaleDateString('en-CA'); // YYYY-MM-DD in local time
    const dayEvents = getEventsForDate(date);
    
    if (dayEvents.length > 0) {
      setShowDateEvents(date);
    } else {
      setNewEvent({ ...newEvent, date: dateStr });
      setSelectedDateForNew(dateStr);
      setShowCreate(true);
    }
  };

  const handleCreateFromDateModal = () => {
    if (showDateEvents) {
      const dateStr = showDateEvents.toLocaleDateString('en-CA');
      setNewEvent({ ...newEvent, date: dateStr });
      setSelectedDateForNew(dateStr);
    }
    setShowDateEvents(null);
    setShowCreate(true);
  };

  if (loading) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-zinc-500">
        <div className="w-24 h-24 rounded-full border-2 border-dashed border-zinc-800 flex items-center justify-center mb-6 animate-pulse">
          <CalendarIcon className="w-8 h-8 opacity-50" />
        </div>
        <p className="text-lg font-medium">Loading calendar...</p>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col gap-4">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-4">
          <h2 className="text-xl font-bold text-white">
            {MONTHS[currentMonth]} {currentYear}
          </h2>
          <div className="flex items-center gap-1">
            <button
              onClick={goToPrevMonth}
              className="p-2 hover:bg-zinc-800 rounded-full transition-colors"
            >
              <ChevronLeft className="w-5 h-5 text-zinc-400" />
            </button>
            <button
              onClick={goToNextMonth}
              className="p-2 hover:bg-zinc-800 rounded-full transition-colors"
            >
              <ChevronRight className="w-5 h-5 text-zinc-400" />
            </button>
          </div>
          <button
            onClick={goToToday}
            className="px-3 py-1.5 text-xs font-medium bg-zinc-800 hover:bg-zinc-700 text-white rounded-full transition-colors"
          >
            Today
          </button>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="bg-white text-black px-5 py-2.5 rounded-full font-bold text-sm hover:bg-zinc-200 transition-colors flex items-center gap-2"
        >
          <Plus className="w-4 h-4" /> Add Event
        </button>
      </div>

      <div className="flex-1 bg-zinc-900/30 border border-zinc-800 rounded-2xl overflow-hidden">
        <div className="grid grid-cols-7 border-b border-zinc-800">
          {DAYS.map(day => (
            <div key={day} className="py-3 text-center text-xs font-medium text-zinc-500 uppercase tracking-wider">
              {day}
            </div>
          ))}
        </div>

        <div className="grid grid-cols-7 flex-1">
          {days.map((day, idx) => {
            if (!day) {
              return <div key={idx} className="min-h-[100px] border-r border-b border-zinc-800/50" />;
            }

            const dayEvents = getEventsForDate(day);
            const today = isToday(day);

            return (
              <div
                key={idx}
                className={`min-h-[100px] border-r border-b border-zinc-800/50 p-1 transition-colors hover:bg-zinc-800/30 cursor-pointer ${
                  today ? 'bg-zinc-800/50' : ''
                }`}
                onClick={() => openCreateForDate(day)}
              >
                <div className={`text-xs font-medium mb-1 w-6 h-6 flex items-center justify-center rounded-full ${
                  today ? 'bg-white text-black' : 'text-zinc-400'
                }`}>
                  {day.getDate()}
                </div>
                <div className="space-y-1">
                  {dayEvents.slice(0, 3).map(event => (
                    <div
                      key={event.id}
                      onClick={(e) => {
                        e.stopPropagation();
                        setShowEventDetails(event);
                      }}
                      className={`text-[10px] px-1.5 py-0.5 rounded truncate cursor-pointer transition-colors flex items-center gap-1 ${
                        event.executed
                          ? 'bg-zinc-700 text-zinc-400'
                          : 'bg-blue-500/20 text-blue-300 hover:bg-blue-500/30'
                      }`}
                    >
                      <ProfilePic src={event.agentPic} name={event.agentName} size="sm" />
                      <span className="truncate">
                        {formatTime12(event.time)} {event.prompt.slice(0, 15)}{event.prompt.length > 15 ? '...' : ''}
                      </span>
                    </div>
                  ))}
                  {dayEvents.length > 3 && (
                    <div className="text-[10px] text-zinc-500 px-1.5">
                      +{dayEvents.length - 3} more
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {events.length > 0 && (
        <div className="mt-4">
          <h3 className="text-sm font-medium text-zinc-400 mb-3 uppercase tracking-wider">All Events</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {events
              .sort((a, b) => {
                const dateA = new Date(`${a.date}T${a.time}`);
                const dateB = new Date(`${b.date}T${b.time}`);
                return dateB.getTime() - dateA.getTime();
              })
              .map(event => (
                <div
                  key={event.id}
                  onClick={() => setShowEventDetails(event)}
                  className={`p-4 rounded-xl border cursor-pointer transition-all hover:scale-[1.02] ${
                    event.executed
                      ? 'bg-zinc-900/50 border-zinc-800'
                      : 'bg-blue-500/5 border-blue-500/20 hover:border-blue-500/40'
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <ProfilePic src={event.agentPic} name={event.agentName} size="md" />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1">
                        <Clock className="w-3 h-3 text-zinc-500" />
                        <span className="text-xs font-mono text-zinc-400">
                          {formatDateDisplay(event.date)} at {formatTime12(event.time)}
                        </span>
                        {event.executed && (
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-500">Done</span>
                        )}
                      </div>
                      <p className="text-sm text-zinc-300 line-clamp-2">{event.prompt}</p>
                      <div className="flex items-center gap-1 mt-2">
                        <span className="text-[10px] text-zinc-500">by {event.agentName}</span>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
          </div>
        </div>
      )}

      {showCreate && (
        <div className="fixed inset-0 bg-black/90 backdrop-blur-md flex items-center justify-center z-50 p-6">
          <div className="bg-zinc-950 border border-zinc-800 w-full max-w-lg rounded-[2.5rem] p-8">
            <div className="flex justify-between items-center mb-6">
              <h3 className="text-2xl font-bold">Create Event</h3>
              <button onClick={() => setShowCreate(false)} className="p-2 hover:bg-zinc-800 rounded-full">
                <X className="w-5 h-5 text-zinc-400" />
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">Agent</label>
                <select
                  value={newEvent.agentId}
                  onChange={e => setNewEvent({ ...newEvent, agentId: parseInt(e.target.value) })}
                  className="w-full bg-black border border-zinc-800 rounded-full px-6 py-3 text-white"
                >
                  <option value={0}>Select agent...</option>
                  {agents?.map(a => (
                    <option key={a.id} value={a.id}>{a.name}</option>
                  ))}
                </select>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">Date</label>
                  <input
                    type="date"
                    value={newEvent.date}
                    onChange={e => setNewEvent({ ...newEvent, date: e.target.value })}
                    className="w-full bg-black border border-zinc-800 rounded-full px-6 py-3 text-white"
                  />
                </div>
                <div>
                  <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">Time</label>
                  <input
                    type="time"
                    value={newEvent.time}
                    onChange={e => setNewEvent({ ...newEvent, time: e.target.value })}
                    className="w-full bg-black border border-zinc-800 rounded-full px-6 py-3 text-white"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">Prompt</label>
                <textarea
                  value={newEvent.prompt}
                  onChange={e => setNewEvent({ ...newEvent, prompt: e.target.value })}
                  placeholder="What should the agent do?"
                  className="w-full bg-black border border-zinc-800 rounded-2xl px-6 py-3 text-white h-24 resize-none"
                />
              </div>
            </div>
            <div className="flex gap-4 mt-6">
              <button onClick={() => setShowCreate(false)} className="flex-1 py-3 text-zinc-400 hover:text-white">Cancel</button>
              <button onClick={handleCreate} className="flex-1 bg-white text-black py-3 rounded-full font-bold">Create Event</button>
            </div>
          </div>
        </div>
      )}

      {showEventDetails && (
        <div className="fixed inset-0 bg-black/90 backdrop-blur-md flex items-center justify-center z-50 p-6">
          <div className="bg-zinc-950 border border-zinc-800 w-full max-w-md rounded-[2.5rem] p-8">
            <div className="flex justify-between items-start mb-6">
              <div className="flex items-center gap-3">
                <ProfilePic src={showEventDetails.agentPic} name={showEventDetails.agentName} size="lg" />
                <div>
                  <h3 className="text-xl font-bold">Event Details</h3>
                  <p className="text-xs text-zinc-500">by {showEventDetails.agentName}</p>
                  {showEventDetails.executed && (
                    <span className="text-xs bg-zinc-800 px-2 py-0.5 rounded text-zinc-400">Executed</span>
                  )}
                </div>
              </div>
              <button onClick={() => setShowEventDetails(null)} className="p-2 hover:bg-zinc-800 rounded-full">
                <X className="w-5 h-5 text-zinc-400" />
              </button>
            </div>
            {editEvent ? (
              <div className="space-y-4">
                <div>
                  <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">Agent</label>
                  <select
                    value={editEvent.agentId}
                    onChange={e => setEditEvent({ ...editEvent, agentId: parseInt(e.target.value) })}
                    className="w-full bg-black border border-zinc-800 rounded-full px-6 py-3 text-white"
                  >
                    {agents?.map(a => (
                      <option key={a.id} value={a.id}>{a.name}</option>
                    ))}
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">Date</label>
                    <input
                      type="date"
                      value={editEvent.date}
                      onChange={e => setEditEvent({ ...editEvent, date: e.target.value })}
                      className="w-full bg-black border border-zinc-800 rounded-full px-6 py-3 text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">Time</label>
                    <input
                      type="time"
                      value={editEvent.time}
                      onChange={e => setEditEvent({ ...editEvent, time: e.target.value })}
                      className="w-full bg-black border border-zinc-800 rounded-full px-6 py-3 text-white"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-zinc-500 uppercase tracking-wider mb-2">Prompt</label>
                  <textarea
                    value={editEvent.prompt}
                    onChange={e => setEditEvent({ ...editEvent, prompt: e.target.value })}
                    className="w-full bg-black border border-zinc-800 rounded-2xl px-6 py-3 text-white h-24 resize-none"
                  />
                </div>
                <div className="flex gap-3 pt-4">
                  <button
                    onClick={() => {
                      setEditEvent(null);
                      setShowEventDetails(null);
                    }}
                    className="flex-1 py-3 text-zinc-400 hover:text-white"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={() => handleUpdate(editEvent.id, editEvent.agentId, editEvent.date, editEvent.time, editEvent.prompt)}
                    className="flex-1 bg-white text-black py-3 rounded-full font-bold"
                  >
                    Save
                  </button>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                <div className="flex items-center gap-3 text-zinc-400">
                  <Clock className="w-4 h-4" />
                  <span className="font-mono">{showEventDetails.date} at {formatTime12(showEventDetails.time)}</span>
                </div>
                <div className="p-4 bg-zinc-900/50 rounded-xl">
                  <p className="text-sm text-zinc-300">{showEventDetails.prompt}</p>
                </div>
                <div className="flex gap-3 pt-4">
                  <button
                    onClick={() => setEditEvent(showEventDetails)}
                    className="flex-1 py-3 bg-zinc-800 hover:bg-zinc-700 text-white rounded-xl transition-colors flex items-center justify-center gap-2"
                  >
                    Edit
                  </button>
                  <button
                    onClick={() => handleDelete(showEventDetails.id)}
                    className="flex-1 py-3 text-red-400 hover:bg-red-400/10 rounded-xl transition-colors flex items-center justify-center gap-2"
                  >
                    <Trash2 className="w-4 h-4" /> Delete
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {showDateEvents && (
        <div className="fixed inset-0 bg-black/90 backdrop-blur-md flex items-center justify-center z-50 p-6">
          <div className="bg-zinc-950 border border-zinc-800 w-full max-w-lg rounded-[2.5rem] p-8 max-h-[80vh] flex flex-col">
            <div className="flex justify-between items-center mb-6">
              <div>
                <h3 className="text-2xl font-bold">
                  {showDateEvents.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' })}
                </h3>
                <p className="text-sm text-zinc-500">{getEventsForDate(showDateEvents).length} events</p>
              </div>
              <button onClick={() => setShowDateEvents(null)} className="p-2 hover:bg-zinc-800 rounded-full">
                <X className="w-5 h-5 text-zinc-400" />
              </button>
            </div>
            
            <div className="flex-1 overflow-y-auto space-y-3 mb-6">
              {getEventsForDate(showDateEvents)
                .sort((a, b) => a.time.localeCompare(b.time))
                .map(event => (
                  <div
                    key={event.id}
                    onClick={() => {
                      setShowDateEvents(null);
                      setShowEventDetails(event);
                    }}
                    className={`p-4 rounded-xl border cursor-pointer transition-all hover:scale-[1.01] ${
                      event.executed
                        ? 'bg-zinc-900/50 border-zinc-800'
                        : 'bg-blue-500/5 border-blue-500/20 hover:border-blue-500/40'
                    }`}
                  >
                    <div className="flex items-start gap-3">
                      <ProfilePic src={event.agentPic} name={event.agentName} size="md" />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <Clock className="w-3 h-3 text-zinc-500" />
                          <span className="text-xs font-mono text-zinc-400">
                            {formatTime12(event.time)}
                          </span>
                          {event.executed && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-500">Done</span>
                          )}
                        </div>
                        <p className="text-sm text-zinc-300 line-clamp-2">{event.prompt}</p>
                        <div className="flex items-center gap-1 mt-2">
                          <span className="text-[10px] text-zinc-500">by {event.agentName}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
            </div>

            <button
              onClick={handleCreateFromDateModal}
              className="w-full bg-white text-black py-3 rounded-full font-bold flex items-center justify-center gap-2 hover:bg-zinc-200 transition-colors"
            >
              <Plus className="w-4 h-4" /> Add Event to This Date
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
