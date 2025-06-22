import React from 'react';

type FeedbackType = 'correct' | 'present' | 'absent' | undefined;

type BoardProps = {
  guesses: string[]; // Each guess is a 5-letter string
  feedback?: FeedbackType[][]; // Array of feedback arrays, one per guess
};

const getTileColor = (fb: FeedbackType) => {
  if (fb === 'correct') return '#538d4e';
  if (fb === 'present') return '#b59f3b';
  if (fb === 'absent') return '#3a3a3c';
  return '#121213';
};

const Board: React.FC<BoardProps> = ({ guesses, feedback = [] }) => {
  const rows = Array.from({ length: 6 }, (_, i) => guesses[i] || '');
  return (
    <div style={{ display: 'grid', gap: 8 }}>
      {rows.map((guess, rowIdx) => (
        <div key={rowIdx} style={{ display: 'flex', gap: 8 }}>
          {Array.from({ length: 5 }, (_, colIdx) => {
            const isFlipped = feedback[rowIdx]?.[colIdx] !== undefined;
            const delay = isFlipped ? `${colIdx * 0.1}s` : '0s';
            return (
              <div
                key={colIdx}
                className={isFlipped ? 'tile flipped' : 'tile'}
                style={{
                  width: 48,
                  height: 48,
                  border: '2px solid #3a3a3c',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '2rem',
                  fontWeight: 700,
                  background: getTileColor(feedback[rowIdx]?.[colIdx]),
                  color: 'white',
                  borderRadius: 6,
                  transition: 'background 0.2s',
                  transform: isFlipped ? 'rotateX(360deg)' : 'none',
                  transitionDelay: delay,
                  transitionProperty: 'background, transform',
                }}
              >
                {guess[colIdx] || ''}
              </div>
            );
          })}
        </div>
      ))}
      {/* Tile flip animation styles */}
      <style>{`
        .tile {
          transform-style: preserve-3d;
          transition: transform 0.8s cubic-bezier(0.4, 0, 0.2, 1), border-color 0.2s ease;
        }
        .tile.flipped {
          transform: rotateX(360deg);
        }
      `}</style>
    </div>
  );
};

export default Board; 