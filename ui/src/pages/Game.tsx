import React, { useState, useEffect } from 'react';
import Board from '../components/Board';
import Keyboard from '../components/Keyboard';
import { getHardModeError } from './hardMode';

const MAX_GUESSES = 6;
const WORD_LENGTH = 5;
const TARGET = 'REACT';
const ALLOWED_WORDS = ['REACT', 'CRANE', 'TRACE', 'LEAST', 'EARTH', 'HEART', 'TEACH', 'CHEAT', 'REACH', 'CARTE'];

type FeedbackType = 'correct' | 'present' | 'absent';

function getFeedback(guess: string, target: string): FeedbackType[] {
  const feedback: FeedbackType[] = Array(WORD_LENGTH).fill('absent');
  const targetArr = target.split('');
  const guessArr = guess.split('');
  const used = Array(WORD_LENGTH).fill(false);

  // First pass: correct
  for (let i = 0; i < WORD_LENGTH; i++) {
    if (guessArr[i] === targetArr[i]) {
      feedback[i] = 'correct';
      used[i] = true;
      targetArr[i] = '';
    }
  }
  // Second pass: present
  for (let i = 0; i < WORD_LENGTH; i++) {
    if (feedback[i] === 'correct') continue;
    const idx = targetArr.findIndex((c) => c === guessArr[i]);
    if (idx !== -1) {
      feedback[i] = 'present';
      targetArr[idx] = '';
    }
  }
  return feedback;
}

const Game: React.FC = () => {
  const [guesses, setGuesses] = useState<string[]>([]);
  const [feedback, setFeedback] = useState<FeedbackType[][]>([]);
  const [current, setCurrent] = useState('');
  const [error, setError] = useState('');
  const [hardMode, setHardMode] = useState(false);
  const [gameOver, setGameOver] = useState(false);
  const [win, setWin] = useState(false);

  // For turn status (single player for now)
  const turnStatus = !gameOver ? 'Your turn!' : '';

  const resetGame = () => {
    setGuesses([]);
    setFeedback([]);
    setCurrent('');
    setError('');
    setHardMode(false);
    setGameOver(false);
    setWin(false);
  };

  useEffect(() => {
    if (feedback.length > 0) {
      const lastFeedback = feedback[feedback.length - 1];
      if (lastFeedback && lastFeedback.every(fb => fb === 'correct')) {
        setGameOver(true);
        setWin(true);
      } else if (guesses.length === MAX_GUESSES) {
        setGameOver(true);
        setWin(false);
      }
    }
  }, [feedback, guesses]);

  const handleKey = (key: string) => {
    if (gameOver) return;
    if (current.length < WORD_LENGTH && /^[A-Z]$/.test(key)) {
      setCurrent(current + key);
      setError('');
    }
  };

  const handleDelete = () => {
    if (gameOver) return;
    setCurrent(current.slice(0, -1));
    setError('');
  };

  const handleEnter = () => {
    if (gameOver) return;
    if (current.length !== WORD_LENGTH) return;
    if (!ALLOWED_WORDS.includes(current)) {
      setError('Not in word list');
      return;
    }
    const hardError = hardMode ? getHardModeError(current, guesses, feedback) : null;
    if (hardError) {
      setError(hardError);
      return;
    }
    if (guesses.length < MAX_GUESSES) {
      setGuesses([...guesses, current]);
      setFeedback([...feedback, getFeedback(current, TARGET)]);
      setCurrent('');
      setError('');
    }
  };

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (gameOver) return;
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
  }, [current, guesses, feedback, gameOver]);

  return (
    <div style={{ minHeight: '100vh', background: '#121213', color: 'white', fontFamily: `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif` }}>
      <div style={{ maxWidth: 500, margin: '0 auto', padding: 20 }}>
        <h1 style={{
          fontSize: 'clamp(1.8rem, 5vw, 2.5rem)',
          marginBottom: '0.5rem',
          background: 'linear-gradient(45deg, #538d4e, #b59f3b)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          textAlign: 'center',
          textShadow: '0 2px 4px rgba(0,0,0,0.1)'
        }}>
          Battle Wordle
        </h1>
        <h2 style={{
          fontSize: 'clamp(1rem, 3vw, 1.2em)',
          color: '#818384',
          marginTop: 0,
          textAlign: 'center',
          fontWeight: 400
        }}>
          Try to avoid guessing the word!
        </h2>
        {/* Turn status */}
        {turnStatus && (
          <div style={{
            textAlign: 'center',
            color: 'white',
            fontSize: 'clamp(1rem, 3vw, 1.2em)',
            margin: '1rem 0',
            padding: '0.75rem',
            background: 'rgba(255,255,255,0.1)',
            borderRadius: 8,
            boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
            transition: 'all 0.3s ease',
          }}>
            {turnStatus}
          </div>
        )}
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
          <Board guesses={[...guesses, current]} feedback={[...feedback, Array(current.length === WORD_LENGTH ? WORD_LENGTH : 0).fill(undefined)]} />
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
            <p style={{ margin: 0, fontSize: 'clamp(0.9rem, 2.5vw, 1.1rem)', lineHeight: 1.5 }}>{win ? 'You win! 🎉' : `You lose!`}</p>
            <p style={{ marginTop: 8, fontWeight: 'bold', fontSize: 'clamp(0.9rem, 2.5vw, 1.1rem)' }}>&quot;{TARGET}&quot;</p>
            <div style={{ display: 'flex', gap: 12, marginTop: 20, justifyContent: 'center' }}>
              <button
                onClick={resetGame}
                style={{
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
                  marginRight: 0,
                }}
                onMouseOver={e => (e.currentTarget.style.background = '#4a7d45')}
                onMouseOut={e => (e.currentTarget.style.background = '#538d4e')}
              >
                Play Again
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