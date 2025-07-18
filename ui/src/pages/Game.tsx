import React, { useState, useEffect, useRef } from 'react';
import Board from '../components/Board';
import Keyboard from '../components/Keyboard';
import { getHardModeError } from './hardMode';
import { useParams, useNavigate } from 'react-router-dom';

const MAX_GUESSES = 6;
const WORD_LENGTH = 5;

// Helper to get player from localStorage
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
  const [hardMode, setHardMode] = useState(false);
  const [loading, setLoading] = useState(true);
  const [gameOver, setGameOver] = useState(false);
  const [win, setWin] = useState(false);
  const receivedGameState = useRef(false);
  const prevGuessesLength = useRef<number>(0);
  const wsRef = useRef<WebSocket | null>(null);
  const [pendingGuess, setPendingGuess] = useState<string | null>(null);
  const player = getPlayer();
  const playerId = player?.id;
  const navigate = useNavigate();

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
      try {
        console.log('WebSocket message received:', event.data);
        const data = JSON.parse(event.data);
        receivedGameState.current = true;
        setGame(data);
        setLoading(false);
        if (data.result) {
          setGameOver(true);
          setWin(data.result === 'win');
        }
        setPendingGuess(null);
      } catch {}
    };
    ws.onerror = (event) => {
      if (!isMounted) return;
      console.warn('WebSocket error:', event);
    };
    ws.onclose = (event) => {
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
    setHardMode(false);
    setGameOver(false);
    setWin(false);
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
    // Placeholder: challenge the opponent to a rematch
    // You can implement the actual challenge logic here
    alert('Rematch feature coming soon!');
  };

  const handleFindMatch = () => {
    navigate('/matchmaking');
  };

  if (loading) {
    return <div style={{ color: 'white', textAlign: 'center', marginTop: '4rem' }}>Loading game...</div>;
  }
  if (error) {
    return <div style={{ color: 'red', textAlign: 'center', marginTop: '4rem' }}>{error}</div>;
  }

  return (
    <div style={{ minHeight: '100vh', background: '#121213', color: 'white', fontFamily: `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif` }}>
      <div style={{ maxWidth: 500, margin: '0 auto', padding: 20 }}>
        {/* Removed Battle Wordle title and subtitle */}
        {/* Hard mode toggle */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16, justifyContent: 'center' }}>
          <label style={{ fontWeight: 500, fontSize: '1.1rem' }}>
            <input
              type="checkbox"
              checked={hardMode}
              onChange={e => setHardMode(e.target.checked)}
              disabled={guesses.length > 0}
              style={{ marginRight: 8 }}
            />
            Hard Mode
          </label>
        </div>
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
            <div style={{ display: 'flex', gap: 12, marginTop: 20, justifyContent: 'center' }}>
              <button
                onClick={() => handleRematch()}
                style={{
                  flex: 1,
                  padding: '12px 24px',
                  border: 'none',
                  borderRadius: 8,
                  cursor: 'pointer',
                  fontSize: '1rem',
                  fontWeight: 'bold',
                  transition: 'all 0.3s ease',
                  minWidth: 120,
                  background: '#538d4e',
                  color: 'white',
                }}
                onMouseOver={e => (e.currentTarget.style.background = '#4a7d45')}
                onMouseOut={e => (e.currentTarget.style.background = '#538d4e')}
              >
                Rematch
              </button>
              <button
                onClick={() => handleFindMatch()}
                style={{
                  flex: 1,
                  padding: '12px 24px',
                  border: 'none',
                  borderRadius: 8,
                  cursor: 'pointer',
                  fontSize: '1rem',
                  fontWeight: 'bold',
                  transition: 'all 0.3s ease',
                  minWidth: 120,
                  background: '#538d4e',
                  color: 'white',
                }}
                onMouseOver={e => (e.currentTarget.style.background = '#4a7d45')}
                onMouseOut={e => (e.currentTarget.style.background = '#538d4e')}
              >
                Find Match
              </button>
            </div>
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