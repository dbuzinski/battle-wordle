import React, { useEffect, useState } from 'react';

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

const mockRecentGames = [
  { id: '1', opponentName: 'WackyDuck', result: 'Won', date: '2024-06-01' },
  { id: '2', opponentName: 'FluffyPanda', result: 'Lost', date: '2024-05-30' },
  { id: '3', opponentName: 'BouncyLlama', result: 'Your Turn', date: '2024-05-29' },
  { id: '4', opponentName: 'GigglyOtter', result: 'Opponent\'s Turn', date: '2024-05-28' },
];

const Home: React.FC = () => {
  const [playerName, setPlayerName] = useState('');
  const [isEditingName, setIsEditingName] = useState(false);
  const [newPlayerName, setNewPlayerName] = useState('');

  useEffect(() => {
    let storedName = localStorage.getItem('playerName');
    if (!storedName) {
      storedName = generateRandomUsername();
      localStorage.setItem('playerName', storedName);
    }
    setPlayerName(storedName);
  }, []);

  const startEditingName = () => {
    setNewPlayerName(playerName);
    setIsEditingName(true);
  };

  const savePlayerName = () => {
    if (newPlayerName.trim()) {
      setPlayerName(newPlayerName.trim());
      localStorage.setItem('playerName', newPlayerName.trim());
      setIsEditingName(false);
    }
  };

  const cancelEditingName = () => {
    setIsEditingName(false);
  };

  const findMatch = () => {
    // TODO: navigate to matchmaking
    alert('Navigate to matchmaking');
  };

  const startNewGame = () => {
    // TODO: navigate to new game
    alert('Navigate to new game');
  };

  return (
    <div style={{ minHeight: '100vh', background: '#121213', color: 'white', fontFamily: `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif` }}>
      <div style={{ maxWidth: 600, margin: '0 auto', padding: '2rem' }}>
        <h1 style={{
          textAlign: 'center',
          fontSize: '2.5rem',
          marginBottom: '2rem',
          background: 'linear-gradient(45deg, #538d4e, #b59f3b)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
        }}>
          Battle Wordle
        </h1>
        {/* Player section */}
        <div style={{ background: 'rgba(255,255,255,0.1)', padding: '1.5rem', borderRadius: 8, marginBottom: '2rem' }}>
          {!isEditingName ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '1.2rem' }}>
              <span>Player: {playerName}</span>
              <button
                style={{ background: 'none', border: '1px solid #538d4e', color: '#538d4e', padding: '0.5rem 1rem', borderRadius: 4, cursor: 'pointer', transition: 'all 0.2s' }}
                onClick={startEditingName}
              >
                Edit
              </button>
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <input
                type="text"
                value={newPlayerName}
                onChange={e => setNewPlayerName(e.target.value)}
                placeholder="Enter your name"
                maxLength={20}
                style={{ padding: '0.5rem', border: '1px solid #3a3a3c', borderRadius: 4, background: '#1a1a1a', color: 'white', fontSize: '1rem' }}
              />
              <div style={{ display: 'flex', gap: '1rem' }}>
                <button
                  style={{ background: '#538d4e', color: 'white', padding: '0.5rem 1rem', borderRadius: 4, border: 'none', fontWeight: 'bold', cursor: 'pointer' }}
                  onClick={savePlayerName}
                >
                  Save
                </button>
                <button
                  style={{ background: '#3a3a3c', color: 'white', padding: '0.5rem 1rem', borderRadius: 4, border: 'none', fontWeight: 'bold', cursor: 'pointer' }}
                  onClick={cancelEditingName}
                >
                  Cancel
                </button>
              </div>
            </div>
          )}
        </div>
        {/* Game actions */}
        <div style={{ display: 'flex', gap: '1rem', marginBottom: '2rem' }}>
          <button
            style={{ flex: 1, padding: '1rem', border: 'none', borderRadius: 8, fontSize: '1.1rem', fontWeight: 'bold', cursor: 'pointer', background: '#538d4e', color: 'white', transition: 'all 0.2s' }}
            onClick={findMatch}
          >
            Find Match
          </button>
          <button
            style={{ flex: 1, padding: '1rem', border: 'none', borderRadius: 8, fontSize: '1.1rem', fontWeight: 'bold', cursor: 'pointer', background: '#538d4e', color: 'white', transition: 'all 0.2s' }}
            onClick={startNewGame}
          >
            New Game
          </button>
        </div>
        {/* Recent games */}
        <div style={{ background: 'rgba(255,255,255,0.1)', padding: '1.5rem', borderRadius: 8 }}>
          <h2 style={{ fontSize: '1.5rem', marginBottom: '1rem', color: '#818384' }}>Recent Games</h2>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0 12px 8px 12px', borderBottom: '1px solid rgba(255,255,255,0.1)', marginBottom: 12 }}>
              <span style={{ color: '#818384', fontSize: '0.9rem', fontWeight: 500, minWidth: 100, flex: 1 }}>Opponent</span>
              <span style={{ color: '#818384', fontSize: '0.9rem', fontWeight: 500, minWidth: 80, textAlign: 'left', padding: '0 4rem' }}>Result</span>
              <span style={{ color: '#818384', fontSize: '0.9rem', fontWeight: 500, minWidth: 80, textAlign: 'right' }}>Date</span>
            </div>
            {mockRecentGames.map(game => (
              <a
                key={game.id}
                href={`#game/${game.id}`}
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: 12,
                  marginBottom: 8,
                  borderRadius: 8,
                  backgroundColor: 'rgba(255,255,255,0.1)',
                  textDecoration: 'none',
                  color: 'white',
                  transition: 'all 0.2s',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', flex: 1, minWidth: 0 }}>
                  <span style={{ fontWeight: 'bold', minWidth: 100, flex: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{game.opponentName}</span>
                  <span style={{
                    padding: '0.25rem 0.5rem',
                    borderRadius: 4,
                    fontSize: '0.9rem',
                    minWidth: 80,
                    textAlign: 'left',
                    display: 'inline-block',
                    margin: '0 4rem',
                    whiteSpace: 'nowrap',
                    color: game.result === 'Won' ? '#538d4e' : game.result === 'Lost' ? '#ff4d4d' : game.result === 'Your Turn' ? '#b59f3b' : '#818384',
                    fontWeight: 'bold',
                  }}>{game.result}</span>
                </div>
                <div style={{ color: '#818384', fontSize: '0.9rem', minWidth: 80, textAlign: 'right', whiteSpace: 'nowrap' }}>{game.date}</div>
              </a>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Home; 