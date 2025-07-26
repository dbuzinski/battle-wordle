import React from 'react';

type KeyboardProps = {
  onKey: (key: string) => void;
  onEnter: () => void;
  onDelete: () => void;
  disabled?: boolean;
};

const KEYS = [
  ['Q','W','E','R','T','Y','U','I','O','P'],
  ['A','S','D','F','G','H','J','K','L'],
  ['Enter','Z','X','C','V','B','N','M','⌫']
];

const Keyboard: React.FC<KeyboardProps> = ({ onKey, onEnter, onDelete, disabled = false }) => {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 24, opacity: disabled ? 0.5 : 1, pointerEvents: disabled ? 'none' : 'auto' }}>
      {KEYS.map((row, rowIdx) => (
        <div key={rowIdx} style={{ display: 'flex', gap: 6, justifyContent: 'center' }}>
          {row.map((key) => (
            <button
              key={key}
              onClick={() => {
                if (key === 'Enter') onEnter();
                else if (key === '⌫') onDelete();
                else onKey(key);
              }}
              disabled={disabled}
              style={{
                minWidth: key === 'Enter' || key === '⌫' ? 56 : 40,
                height: 48,
                background: '#818384',
                color: 'white',
                border: 'none',
                borderRadius: 6,
                fontSize: '1.1rem',
                fontWeight: 600,
                cursor: disabled ? 'not-allowed' : 'pointer',
                padding: '0 8px',
                opacity: disabled ? 0.7 : 1,
              }}
            >
              {key}
            </button>
          ))}
        </div>
      ))}
    </div>
  );
};

export default Keyboard; 