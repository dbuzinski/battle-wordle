import React, { useState, useEffect, useRef } from 'react';
import { Routes, Route, useLocation } from 'react-router-dom';
import Home from './pages/Home';
import Game from './pages/Game';
import Matchmaking from './pages/Matchmaking';
import ChallengePage from './pages/Challenge';
import Leaderboard from './pages/Leaderboard';
import NavBar from './components/NavBar';
import ChallengePopover from './components/ChallengePopover';
import type { Challenge as ChallengeNotification } from './components/ChallengePopover';
import './App.css';
import { NotificationWebSocketProvider } from './components/NotificationWebSocketContext';
import { useNavigate } from 'react-router-dom';

// Define the Player type
interface Player {
  id: string;
  name: string;
  registered: boolean;
  elo?: number;
  created_at: string;
}

function App() {
  const navigate = useNavigate();
  const location = useLocation();
  const [player, setPlayer] = useState<Player | null>(null);
  const [showLoginModal, setShowLoginModal] = useState(false);
  const [showRegister, setShowRegister] = useState(false);
  const [registerForm, setRegisterForm] = useState({ name: '', password: '', confirm: '' });
  const [loginForm, setLoginForm] = useState({ name: '', password: '' });
  const [authError, setAuthError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Challenge popover state
  const [challenges, setChallenges] = useState<ChallengeNotification[]>([]);

  // Rematch popover state
  const [rematchOffer, setRematchOffer] = useState<null | {
    from: string;
    fromName: string;
    prevGameId: string;
  }>(null);

  // Global WebSocket for challenge notifications
  const [ws, setWs] = useState<WebSocket | null>(null);
  useEffect(() => {
    if (!player || !player.id) return;
    const wsInstance = new WebSocket(
      window.location.protocol === 'https:'
        ? `wss://${window.location.host}/ws/notifications`
        : `ws://${window.location.host}/ws/notifications`
    );
    setWs(wsInstance);
    wsInstance.onopen = () => {
      console.log('[Notification WS] Connected');
      wsInstance.send(JSON.stringify({ type: 'join', player_id: player.id }));
    };
    wsInstance.onmessage = (event) => {
      console.log('[Notification WS] Message:', event.data);
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'challenge_invite' && msg.from) {
          // Only suppress rematch popover if:
          // - it's a rematch
          // - we're on the relevant game page
          // - the challenger is the opponent from that game
          const isRematch = !!msg.rematch;
          if (
            isRematch &&
            location.pathname.startsWith('/game/')
          ) {
            const currentOpponentId = sessionStorage.getItem('currentGameOpponentId');
            if (currentOpponentId && msg.from === currentOpponentId) {
              // Suppress popover, let Game.tsx handle
              return;
            }
          }
          setChallenges((prev) => [
            ...prev,
            {
              id: `${msg.from}-${isRematch ? 'rematch-' : ''}-${Date.now()}`,
              fromName: msg.from_name || msg.from,
              fromId: msg.from,
              rematch: isRematch,
            },
          ]);
        }
        if (msg.type === 'challenge_result') {
          console.log('[Notification WS] challenge_result:', msg);
          if (msg.accepted && msg.game_id) {
            if (player && (msg.to === player.id || msg.from === player.id)) {
              navigate(`/game/${msg.game_id}`);
            }
          }
        }
        // Handle other notification types here
      } catch {}
    };
    wsInstance.onerror = () => {};
    return () => {
      wsInstance.close();
      setWs(null);
    };
  }, [player, location.pathname]);

  const handleAcceptChallenge = (id: string) => {
    setChallenges((prev) => prev.filter((c) => c.id !== id));
    const challenge = challenges.find((c) => c.id === id);
    if (!challenge) return;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type: 'challenge_response',
        from: player?.id,
        to: challenge.fromId,
        accepted: true,
        rematch: challenge.rematch,
      }));
    }
  };
  const handleRejectChallenge = (id: string) => {
    setChallenges((prev) => prev.filter((c) => c.id !== id));
    const challenge = challenges.find((c) => c.id === id);
    if (!challenge) return;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type: 'challenge_response',
        from: player?.id,
        to: challenge.fromId,
        accepted: false,
        rematch: challenge.rematch,
      }));
    }
  };

  // Accept/Reject rematch handlers
  const handleAcceptRematch = () => {
    if (!rematchOffer || !ws) return;
    ws.send(JSON.stringify({
      type: 'rematch_response',
      from: player?.id,
      to: rematchOffer.from,
      accepted: true,
    }));
    setRematchOffer(null);
  };
  const handleRejectRematch = () => {
    if (!rematchOffer || !ws) return;
    ws.send(JSON.stringify({
      type: 'rematch_response',
      from: player?.id,
      to: rematchOffer.from,
      accepted: false,
    }));
    setRematchOffer(null);
  };

  // Register guest player on first visit
  useEffect(() => {
    const storedPlayer = localStorage.getItem('player');
    if (storedPlayer) {
      setPlayer(JSON.parse(storedPlayer));
      setLoading(false);
      return;
    }
    // No player found, register a guest
    const adjectives = [
      'Silly', 'Bouncy', 'Wiggly', 'Giggly', 'Wobbly', 'Fluffy', 'Bumpy', 'Jumpy',
      'Wacky', 'Zippy', 'Dizzy', 'Fuzzy', 'Squishy', 'Bouncy', 'Wiggly', 'Giggly'
    ];
    const nouns = [
      'Panda', 'Penguin', 'Duck', 'Koala', 'Sloth', 'Otter', 'Puffin', 'Quokka',
      'Narwhal', 'Platypus', 'Axolotl', 'Capybara', 'Flamingo', 'Meerkat', 'Wombat', 'Llama'
    ];
    function generateRandomUsername() {
      const randomAdj = adjectives[Math.floor(Math.random() * adjectives.length)];
      const randomNoun = nouns[Math.floor(Math.random() * nouns.length)];
      return `${randomAdj}${randomNoun}`;
    }
    const randomName = generateRandomUsername();
    fetch('/api/player/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: randomName })
    })
      .then(async (res) => {
        if (!res.ok) throw new Error('Failed to register player');
        const data = await res.json();
        setPlayer(data);
        localStorage.setItem('player', JSON.stringify(data));
        setLoading(false);
      })
      .catch((err) => {
        setError('Could not register player. Please try again.');
        setLoading(false);
      });
  }, []);

  // Modal handlers
  const openLoginModal = () => {
    setLoginForm({ name: '', password: '' });
    setRegisterForm({ name: player?.name || '', password: '', confirm: '' });
    setAuthError(null);
    setShowLoginModal(true);
    setShowRegister(false);
  };
  const closeLoginModal = () => setShowLoginModal(false);

  // Auth handlers (login/register)
  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthError(null);
    if (!registerForm.name.trim() || !registerForm.password || registerForm.password !== registerForm.confirm) {
      setAuthError('Please fill all fields and make sure passwords match.');
      return;
    }
    try {
      const res = await fetch('/api/player/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: player?.id,
          name: registerForm.name.trim(),
          password: registerForm.password
        })
      });
      if (res.status === 409) {
        // Username taken
        const data = await res.json();
        setAuthError(data.error || 'Username is already taken.');
        return;
      }
      if (!res.ok) throw new Error('Registration failed');
      const data = await res.json();
      setPlayer(data.player);
      localStorage.setItem('player', JSON.stringify(data.player));
      localStorage.setItem('token', data.token);
      setShowLoginModal(false);
    } catch (err) {
      setAuthError('Registration failed. Try a different name.');
    }
  };
  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthError(null);
    if (!loginForm.name.trim() || !loginForm.password) {
      setAuthError('Please fill all fields.');
      return;
    }
    try {
      const res = await fetch('/api/player/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: loginForm.name.trim(),
          password: loginForm.password
        })
      });
      if (!res.ok) throw new Error('Login failed');
      const data = await res.json();
      setPlayer(data.player);
      localStorage.setItem('player', JSON.stringify(data.player));
      localStorage.setItem('token', data.token);
      setShowLoginModal(false);
    } catch (err) {
      setAuthError('Login failed. Check your credentials.');
    }
  };
  const handleLogout = () => {
    setPlayer(null);
    localStorage.removeItem('player');
    window.location.reload();
  };

  if (loading) {
    return <div style={{ color: 'white', textAlign: 'center', marginTop: '4rem' }}>Loading...</div>;
  }
  if (error) {
    return <div style={{ color: 'red', textAlign: 'center', marginTop: '4rem' }}>{error}</div>;
  }

  return (
    <NotificationWebSocketProvider ws={ws}>
      <NavBar
        isLoggedIn={!!player?.registered}
        playerName={player?.name}
        onLoginClick={openLoginModal}
        onLogoutClick={handleLogout}
      />
      {/* Global Challenge Popover */}
      <ChallengePopover
        challenges={challenges}
        onAccept={handleAcceptChallenge}
        onReject={handleRejectChallenge}
      />
      {/* Login/Register Modal (global) */}
      {showLoginModal && (
        <div style={{ position: 'fixed', top: 0, left: 0, width: '100vw', height: '100vh', background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <form onSubmit={showRegister ? handleRegister : handleLogin} style={{ background: '#222', padding: '2rem', borderRadius: 8, minWidth: 320, boxShadow: '0 2px 16px #0008', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <h2 style={{ color: '#f3f3f3', margin: 0, textAlign: 'center' }}>{showRegister ? 'Create Account' : 'Login'}</h2>
            {showRegister ? (
              <>
                <input type="text" value={registerForm.name} onChange={e => setRegisterForm(f => ({ ...f, name: e.target.value }))} placeholder="Username" maxLength={20} style={{ padding: '0.5rem', borderRadius: 4, border: '1px solid #3a3a3c', background: '#1a1a1a', color: '#f3f3f3' }} />
                <input type="password" value={registerForm.password} onChange={e => setRegisterForm(f => ({ ...f, password: e.target.value }))} placeholder="Password" style={{ padding: '0.5rem', borderRadius: 4, border: '1px solid #3a3a3c', background: '#1a1a1a', color: '#f3f3f3' }} />
                <input type="password" value={registerForm.confirm} onChange={e => setRegisterForm(f => ({ ...f, confirm: e.target.value }))} placeholder="Confirm Password" style={{ padding: '0.5rem', borderRadius: 4, border: '1px solid #3a3a3c', background: '#1a1a1a', color: '#f3f3f3' }} />
              </>
            ) : (
              <>
                <input type="text" value={loginForm.name} onChange={e => setLoginForm(f => ({ ...f, name: e.target.value }))} placeholder="Username" maxLength={20} style={{ padding: '0.5rem', borderRadius: 4, border: '1px solid #3a3a3c', background: '#1a1a1a', color: '#f3f3f3' }} />
                <input type="password" value={loginForm.password} onChange={e => setLoginForm(f => ({ ...f, password: e.target.value }))} placeholder="Password" style={{ padding: '0.5rem', borderRadius: 4, border: '1px solid #3a3a3c', background: '#1a1a1a', color: '#f3f3f3' }} />
              </>
            )}
            {authError && <div style={{ color: 'red', fontSize: '0.95rem' }}>{authError}</div>}
            <div style={{ display: 'flex', gap: '1rem', justifyContent: 'flex-end', marginTop: 8 }}>
              <button type="button" onClick={closeLoginModal} style={{ background: '#3a3a3c', color: '#f3f3f3', border: 'none', borderRadius: 4, padding: '0.5rem 1rem', cursor: 'pointer' }}>Cancel</button>
              <button type="submit" style={{ background: '#538d4e', color: '#f3f3f3', border: 'none', borderRadius: 4, padding: '0.5rem 1rem', fontWeight: 'bold', cursor: 'pointer' }}>{showRegister ? 'Sign Up' : 'Login'}</button>
            </div>
            <div style={{ textAlign: 'center', marginTop: 8 }}>
              {showRegister ? (
                <button type="button" onClick={() => setShowRegister(false)} style={{ background: 'none', border: 'none', color: '#f3f3f3', cursor: 'pointer', textDecoration: 'underline', fontSize: '1rem' }}>
                  Already have an account? Login
                </button>
              ) : (
                <button type="button" onClick={() => setShowRegister(true)} style={{ background: 'none', border: 'none', color: '#f3f3f3', cursor: 'pointer', textDecoration: 'underline', fontSize: '1rem' }}>
                  Don&apos;t have an account? Sign up
                </button>
              )}
            </div>
          </form>
        </div>
      )}
      <Routes>
        <Route path="/" element={<Home player={player} setPlayer={setPlayer} />} />
        <Route path="/game/:id" element={<Game />} />
        <Route path="/matchmaking" element={<Matchmaking />} />
        <Route path="/challenge" element={<ChallengePage />} />
        <Route path="/leaderboard" element={<Leaderboard />} />
      </Routes>
    </NotificationWebSocketProvider>
  );
}

export default App;
