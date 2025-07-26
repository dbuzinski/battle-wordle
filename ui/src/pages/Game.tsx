import React, { useState, useEffect, useRef } from 'react';
import Board from '../components/Board';
import Keyboard from '../components/Keyboard';
import { useParams, useNavigate } from 'react-router-dom';
import { useNotificationWebSocket } from '../components/NotificationWebSocketContext';

const MAX_GUESSES = 6;
const WORD_LENGTH = 5;

function getPlayer() {
  try {
    return JSON.parse(localStorage.getItem('player') || 'null');
  } catch {
    return null;
  }
}

const Game: React.FC = () => {
  const { id: gameId } = useParams<{ id: string }>();
  const [game, setGame] = useState<any>(null);
  const [current, setCurrent] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [gameOver, setGameOver] = useState(false);
  const receivedGameState = useRef(false);
  const prevGuessesLength = useRef<number>(0);
  const wsRef = useRef<WebSocket | null>(null);
  const [pendingGuess, setPendingGuess] = useState<string | null>(null);
  const player = getPlayer();
  const playerId = player?.id;
  const navigate = useNavigate();
  const [opponentStats, setOpponentStats] = useState<{ wins: number; losses: number; draws: number }>({ wins: 0, losses: 0, draws: 0 });
  const notificationWS = useNotificationWebSocket();
  // Rematch state (unified with challenge flow)
  const [rematchState, setRematchState] = useState<'idle' | 'pending' | 'offered' | 'declined' | 'accepted'>('idle');
  const [rematchFrom, setRematchFrom] = useState<string | null>(null);
  const [rematchOfferId, setRematchOfferId] = useState<string | null>(null);

  // Define opponent for display, stats, and sessionStorage
  type Opponent = { id: string; name: string };
  let opponent: Opponent | null = null;
  if (game && player) {
    if (game.first_player.id === player.id) {
      opponent = game.second_player;
    } else {
      opponent = game.first_player;
    }
  }

  useEffect(() => {
    let isMounted = true;
    if (!gameId) return;
    setLoading(true);
    receivedGameState.current = false;
    const ws = new WebSocket(
      window.location.protocol === 'https:'
        ? `wss://${window.location.host}/ws/game/${gameId}`
        : `ws://${window.location.host}/ws/game/${gameId}`
    );
    wsRef.current = ws;
    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'join' }));
    };
    ws.onmessage = (event) => {
      if (!isMounted) return;
      console.log('WebSocket message received:', event.data);
      const data = JSON.parse(event.data);
      receivedGameState.current = true;
      setGame(data);
      setLoading(false);
      if (data.result) {
        setGameOver(true);
      }
      setPendingGuess(null);
      // Set sessionStorage for opponent id as soon as game state is received
      let opponentId = null;
      if (player && data.first_player && data.second_player) {
        if (data.first_player.id === player.id) {
          opponentId = data.second_player.id;
        } else {
          opponentId = data.first_player.id;
        }
      }
      if (opponentId) {
        sessionStorage.setItem('currentGameOpponentId', opponentId);
      } else {
        sessionStorage.removeItem('currentGameOpponentId');
      }
    };
    ws.onerror = (event) => {
      if (!isMounted) return;
      console.warn('WebSocket error:', event);
    };
    ws.onclose = () => {
      if (!isMounted) return;
      if (!receivedGameState.current) {
        setError('WebSocket connection closed before receiving game state.');
        setLoading(false);
      }
    };
    return () => {
      isMounted = false;
      ws.close();
      wsRef.current = null;
    };
  }, [gameId]);

  useEffect(() => {
    setGameOver(false);
    setError('');
    setCurrent('');
    setPendingGuess(null);
    // Optionally reset other state if needed
  }, [gameId]);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (gameOver || pendingGuess) return;
      if (!playerId || game?.current_player !== playerId) return;
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      const key = e.key.toUpperCase();
      if (/^[A-Z]$/.test(key)) {
        handleKey(key);
      } else if (e.key === 'Enter') {
        handleEnter();
      } else if (e.key === 'Backspace' || e.key === 'Delete') {
        handleDelete();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [gameOver, current, pendingGuess, playerId, game?.current_player]);

  useEffect(() => {
    if (!game) return;
    if (game.guesses.length > prevGuessesLength.current) {
      setCurrent('');
    }
    prevGuessesLength.current = game.guesses.length;
  }, [game?.guesses.length]);

  const guesses = game?.guesses || [];
  const feedback = Array.isArray(game?.feedback) ? game.feedback : [];

  const resetGame = () => {
    setCurrent('');
    setError('');
    setGameOver(false);
  };

  const handleKey = (key: string) => {
    if (gameOver) return;
    if (!playerId || game?.current_player !== playerId) return;
    if (current.length < 5 && /^[A-Z]$/.test(key)) {
      setCurrent(prev => prev + key);
      setError('');
    }
  };

  const handleDelete = () => {
    if (gameOver) return;
    if (!playerId || game?.current_player !== playerId) return;
    setCurrent(current.slice(0, -1));
    setError('');
  };

  const handleEnter = () => {
    if (gameOver) return;
    if (!playerId || game?.current_player !== playerId) return;
    if (current.length !== 5) return;
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      setError('WebSocket not connected.');
      return;
    }
    try {
      wsRef.current.send(JSON.stringify({ type: 'guess', guess: current, player_id: playerId }));
      setError('');
      setPendingGuess(current);
      setCurrent('');
      // The game state will be updated via WebSocket message
    } catch (err) {
      setError('Failed to send guess.');
    }
  };

  let displayGuesses = guesses;
  let displayFeedback = feedback;
  if (!gameOver && pendingGuess && guesses.length < MAX_GUESSES && !guesses.includes(pendingGuess)) {
    displayGuesses = [...guesses, pendingGuess];
    displayFeedback = [...feedback, Array(WORD_LENGTH).fill(undefined)];
  } else if (!gameOver && current.length > 0 && guesses.length < MAX_GUESSES) {
    displayGuesses = [...guesses, current];
    displayFeedback = [...feedback, Array(current.length === WORD_LENGTH ? WORD_LENGTH : 0).fill(undefined)];
  }

  const handleRematch = () => {
    if (!game || !player || !opponent || !notificationWS || notificationWS.readyState !== WebSocket.OPEN) {
      alert('Rematch connection not available.');
      return;
    }
    setRematchState('pending');
    setRematchOfferId(game.id);
    notificationWS.send(JSON.stringify({
      type: 'challenge_invite',
      from: player.id,
      to: opponent.id,
      rematch: true,
      from_name: player.name,
    }));
    console.log('[Rematch] Sent challenge_invite (rematch)', { from: player.id, to: opponent.id });
  };

  const handleCancelRematch = () => {
    if (!game || !player || !opponent || !notificationWS || notificationWS.readyState !== WebSocket.OPEN) return;
    setRematchState('idle');
    setRematchOfferId(null);
    notificationWS.send(JSON.stringify({
      type: 'challenge_cancel',
      from: player.id,
      to: opponent.id,
      rematch: true,
    }));
    console.log('[Rematch] Sent challenge_cancel (rematch)', { from: player.id, to: opponent.id });
  };

  // Listen for challenge_invite, challenge_cancel, challenge_response, challenge_result (rematch) on notificationWS
  useEffect(() => {
    if (!notificationWS) return;
    const handler = (event: MessageEvent) => {
      console.log('WebSocket event:', event.data);
      try {
        const msg = JSON.parse(event.data);
        console.log('Parsed message:', msg);
        if (
          msg.type === 'challenge_invite' &&
          msg.rematch &&
          msg.to === playerId
        ) {
          setRematchState('offered');
          setRematchFrom(msg.from_name || msg.from);
          setRematchOfferId('rematch');
          console.log('[Rematch] Received challenge_invite (rematch)', msg);
        } else if ((msg.type === 'challenge_cancel' || msg.type === 'challenge_cancelled') && msg.rematch && (msg.to === playerId || msg.type === 'challenge_cancelled')) {
          setRematchState('idle');
          setRematchFrom(null);
          setRematchOfferId(null);
          console.log('[Rematch] Received challenge_cancel (rematch)', msg);
        } else if (msg.type === 'challenge_response' && msg.rematch && msg.to === playerId && msg.accepted === false) {
          setRematchState('declined');
          setTimeout(() => setRematchState('idle'), 2000);
          setRematchOfferId(null);
          setRematchFrom(null);
          console.log('[Rematch] Received challenge_response declined (rematch)', msg);
        } else if (msg.type === 'challenge_result' && msg.rematch && msg.game_id) {
          setRematchState('accepted');
          setRematchOfferId(null);
          setRematchFrom(null);
          // navigation handled globally
        }
      } catch {}
    };
    notificationWS.addEventListener('message', handler);
    return () => notificationWS.removeEventListener('message', handler);
  }, [notificationWS, playerId]);

  const handleAcceptRematch = () => {
    if (!game || !player || !opponent || !notificationWS || notificationWS.readyState !== WebSocket.OPEN) return;
    notificationWS.send(JSON.stringify({
      type: 'challenge_response',
      from: player.id,
      to: opponent.id,
      accepted: true,
      rematch: true,
    }));
    setRematchState('accepted');
    setRematchOfferId(null);
    setRematchFrom(null);
    console.log('[Rematch] Sent challenge_response accepted (rematch)', { from: player.id, to: opponent.id });
  };
  const handleRejectRematch = () => {
    if (!game || !player || !opponent || !notificationWS || notificationWS.readyState !== WebSocket.OPEN) return;
    notificationWS.send(JSON.stringify({
      type: 'challenge_response',
      from: player.id,
      to: opponent.id,
      accepted: false,
      rematch: true,
    }));
    setRematchState('idle');
    setRematchOfferId(null);
    setRematchFrom(null);
    console.log('[Rematch] Sent challenge_response declined (rematch)', { from: player.id, to: opponent.id });
  };

  const handleFindMatch = () => {
    navigate('/matchmaking');
  };

  useEffect(() => {
    // Set the current opponent id in sessionStorage for App.tsx to use
    if (opponent && opponent.id) {
      sessionStorage.setItem('currentGameOpponentId', opponent.id);
    } else {
      sessionStorage.removeItem('currentGameOpponentId');
    }
  }, [opponent?.id]);

  if (loading) {
    return <div style={{ color: 'white', textAlign: 'center', marginTop: '4rem' }}>Loading game...</div>;
  }
  if (error) {
    return <div style={{ color: 'red', textAlign: 'center', marginTop: '4rem' }}>{error}</div>;
  }

  return (
    <div style={{ minHeight: '100vh', background: '#121213', color: 'white', fontFamily: `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif` }}>
      <div style={{ maxWidth: 500, margin: '0 auto', padding: 20 }}>
        {/* Opponent Info - card layout with stats */}
        {game && !gameOver && opponent && (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginBottom: 24 }}>
            <div style={{
              background: 'rgba(255,255,255,0.08)',
              borderRadius: 10,
              padding: '18px 28px',
              minWidth: 220,
              boxShadow: '0 2px 8px rgba(0,0,0,0.10)',
              textAlign: 'center',
              marginBottom: 8,
            }}>
              <div style={{ fontSize: '1.18rem', fontWeight: 500, marginBottom: 6 }}>vs <b>{opponent.name}</b></div>
              {opponentStats && (
                <div style={{ fontSize: '0.98rem', marginBottom: 4, color: '#bdbdbd', display: 'flex', justifyContent: 'center', gap: 16 }}>
                  <span>W: {opponentStats.wins}</span>
                  <span>L: {opponentStats.losses}</span>
                  <span>D: {opponentStats.draws}</span>
                </div>
              )}
              <div style={{ fontSize: '1.08rem', marginTop: 10, fontWeight: 500 }}>
                {game.current_player === playerId ? (
                  <span>Your turn!</span>
                ) : (
                  <span>Waiting for opponent...</span>
                )}
              </div>
            </div>
          </div>
        )}
        {/* Board - no card background */}
        <div style={{
          margin: '2rem 0',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
        }}>
          <Board guesses={displayGuesses} feedback={displayFeedback} />
        </div>
        {/* Error message modal */}
        {error && !gameOver && (
          <div
            style={{
              position: 'fixed',
              top: '50%',
              left: '50%',
              transform: 'translate(-50%, -50%)',
              padding: '12px 24px',
              borderRadius: 8,
              background: 'rgba(255,255,255,0.95)',
              color: '#121213',
              zIndex: 999,
              textAlign: 'center',
              boxShadow: '0 4px 12px rgba(0,0,0,0.2)',
              animation: 'slideDown 0.3s ease-out',
              fontSize: 'clamp(0.9rem, 2.5vw, 1.1rem)',
            }}
          >
            {error}
          </div>
        )}
        {/* Game over modal */}
        {gameOver && (
          <div
            style={{
              position: 'fixed',
              top: '50%',
              left: '50%',
              transform: 'translate(-50%, -50%)',
              padding: 24,
              borderRadius: 8,
              background: 'rgba(255,255,255,0.95)',
              color: '#121213',
              zIndex: 999,
              textAlign: 'center',
              boxShadow: '0 4px 12px rgba(0,0,0,0.2)',
              animation: 'slideDown 0.3s ease-out',
              backdropFilter: 'blur(8px)',
              minWidth: 280,
            }}
          >
            {(() => {
              if (game?.result === 'draw') {
                return <p style={{ margin: 0, fontSize: 'clamp(0.9rem, 2.5vw, 1.1rem)', lineHeight: 1.5 }}>Draw!</p>;
              } else if (typeof game?.result === 'string' && game.result.startsWith('lose:')) {
                const loserId = game.result.split(':')[1];
                if (playerId && loserId === playerId) {
                  return <p style={{ margin: 0, fontSize: 'clamp(0.9rem, 2.5vw, 1.1rem)', lineHeight: 1.5 }}>You lose!</p>;
                } else {
                  return <p style={{ margin: 0, fontSize: 'clamp(0.9rem, 2.5vw, 1.1rem)', lineHeight: 1.5 }}>You win! 🎉</p>;
                }
              } else {
                return <p style={{ margin: 0, fontSize: 'clamp(0.9rem, 2.5vw, 1.1rem)', lineHeight: 1.5 }}>Game Over</p>;
              }
            })()}
            {game?.solution && (
              <p style={{ marginTop: 8, fontWeight: 'bold', fontSize: 'clamp(0.9rem, 2.5vw, 1.1rem)' }}>&quot;{game.solution}&quot;</p>
            )}
            {/* Rematch logic in Game Over popover */}
            {rematchState === 'offered' ? (
              (() => { console.log('Rendering rematch offer UI'); return null; })() ||
              <div style={{ marginTop: 18 }}>
                <div style={{ fontWeight: 500, marginBottom: 10 }}>{rematchFrom ? `${rematchFrom} wants a rematch!` : 'Rematch?'}</div>
                <div style={{ display: 'flex', gap: 12, justifyContent: 'center' }}>
                  <button
                    onClick={handleAcceptRematch}
                    style={{ padding: '10px 24px', borderRadius: 8, border: 'none', background: '#538d4e', color: 'white', fontWeight: 600, fontSize: 16, cursor: 'pointer' }}
                  >Accept</button>
                  <button
                    onClick={handleRejectRematch}
                    style={{ padding: '10px 24px', borderRadius: 8, border: 'none', background: '#f44336', color: 'white', fontWeight: 600, fontSize: 16, cursor: 'pointer' }}
                  >Reject</button>
                </div>
              </div>
            ) : rematchState === 'declined' ? (
              <div style={{ marginTop: 18, color: '#f44336', fontWeight: 500 }}>Rematch declined</div>
            ) : (
              <div style={{ display: 'flex', gap: 12, marginTop: 20, justifyContent: 'center' }}>
                {rematchState === 'pending' ? (
                  <button
                    onClick={handleCancelRematch}
                    style={{ flex: 1, padding: '12px 24px', border: 'none', borderRadius: 8, cursor: 'pointer', fontSize: '1rem', fontWeight: 'bold', transition: 'all 0.3s ease', minWidth: 120, background: '#888', color: 'white' }}
                  >Cancel</button>
                ) : (
                  <button
                    onClick={handleRematch}
                    style={{ flex: 1, padding: '12px 24px', border: 'none', borderRadius: 8, cursor: 'pointer', fontSize: '1rem', fontWeight: 'bold', transition: 'all 0.3s ease', minWidth: 120, background: '#538d4e', color: 'white' }}
                    onMouseOver={e => (e.currentTarget.style.background = '#4a7d45')}
                    onMouseOut={e => (e.currentTarget.style.background = '#538d4e')}
                  >Rematch</button>
                )}
                <button
                  onClick={handleFindMatch}
                  style={{ flex: 1, padding: '12px 24px', border: 'none', borderRadius: 8, cursor: 'pointer', fontSize: '1rem', fontWeight: 'bold', transition: 'all 0.3s ease', minWidth: 120, background: '#538d4e', color: 'white' }}
                  onMouseOver={e => (e.currentTarget.style.background = '#4a7d45')}
                  onMouseOut={e => (e.currentTarget.style.background = '#538d4e')}
                >Find Match</button>
              </div>
            )}
          </div>
        )}
        {/* Keyboard - no card background */}
        <div style={{
          margin: '2rem 0',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
        }}>
          <Keyboard onKey={handleKey} onEnter={handleEnter} onDelete={handleDelete} disabled={gameOver} />
        </div>
      </div>
    </div>
  );
};

export default Game; 
