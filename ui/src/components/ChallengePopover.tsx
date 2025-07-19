import React from "react";

export interface Challenge {
  id: string;
  fromName: string;
  fromId: string;
  rematch?: boolean;
  prevGameId?: string;
}

interface ChallengePopoverProps {
  challenges: Challenge[];
  onAccept: (id: string) => void;
  onReject: (id: string) => void;
}

const popoverStyle: React.CSSProperties = {
  position: "fixed",
  bottom: 24,
  right: 24,
  zIndex: 1000,
  minWidth: 240,
  maxWidth: 320,
  background: "rgba(28, 28, 36, 0.98)",
  borderRadius: 12,
  boxShadow: "0 6px 32px rgba(0,0,0,0.18)",
  padding: 0,
  color: "#fff",
  fontFamily: "'Inter', 'Segoe UI', Arial, sans-serif",
  border: "1.5px solid #23233a",
  overflow: 'hidden',
};

const headerStyle: React.CSSProperties = {
  fontWeight: 700,
  fontSize: 15.5,
  letterSpacing: 0.2,
  padding: "12px 18px 8px 18px",
  background: "rgba(36,36,48,0.98)",
  borderBottom: "1px solid #23233a",
  textAlign: 'left',
};

const challengeListStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 0,
  padding: 0,
  margin: 0,
};

const challengeStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  background: "none",
  borderBottom: "1px solid #23233a",
  padding: "10px 18px",
  fontSize: 15.5,
  transition: "background 0.18s",
  minHeight: 38,
};

const nameStyle: React.CSSProperties = {
  fontWeight: 500,
  fontSize: 15.5,
  marginRight: 10,
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  maxWidth: 140,
  color: '#e3e3e3',
  letterSpacing: 0.1,
};

const buttonStyle: React.CSSProperties = {
  border: "none",
  background: "none",
  color: "#fff",
  fontSize: 18,
  cursor: "pointer",
  marginLeft: 6,
  padding: 2,
  borderRadius: 4,
  transition: "background 0.15s, color 0.15s",
};

const ChallengePopover: React.FC<ChallengePopoverProps> = ({ challenges, onAccept, onReject }) => {
  if (challenges.length === 0) return null;
  return (
    <div style={popoverStyle}>
      <div style={headerStyle}>Challenges</div>
      <div style={challengeListStyle}>
        {challenges.map((c, idx) => (
          <div key={c.id} style={{ ...challengeStyle, borderBottom: idx === challenges.length - 1 ? 'none' : challengeStyle.borderBottom }}>
            <span style={nameStyle}>{c.fromName} {c.rematch ? '(Rematch)' : ''}</span>
            <span>
              <button
                style={{ ...buttonStyle, color: "#4caf50" }}
                title="Accept"
                onClick={() => onAccept(c.id)}
                onMouseOver={e => (e.currentTarget.style.background = "#2e4")}
                onMouseOut={e => (e.currentTarget.style.background = "none")}
              >
                ✔️
              </button>
              <button
                style={{ ...buttonStyle, color: "#f44336" }}
                title="Reject"
                onClick={() => onReject(c.id)}
                onMouseOver={e => (e.currentTarget.style.background = "#e44")}
                onMouseOut={e => (e.currentTarget.style.background = "none")}
              >
                ❌
              </button>
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default ChallengePopover; 