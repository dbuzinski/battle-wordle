import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { FaPencilAlt } from 'react-icons/fa';

// Define the Player type
interface Player {
  id: string;
  name: string;
  registered: boolean;
  created_at: string;
}

interface HomeProps {
  player: Player | null;
  setPlayer: React.Dispatch<React.SetStateAction<Player | null>>;
}

const Home: React.FC<HomeProps> = ({ player, setPlayer }) => {
  const [isEditingName, setIsEditingName] = useState(false);
  const [newPlayerName, setNewPlayerName] = useState('');
  const navigate = useNavigate();
  const [recentGames, setRecentGames] = useState<any[] | null>(null);
  const [gamesLoading, setGamesLoading] = useState(false);
  const [gamesError, setGamesError] = useState<string | null>(null);

  useEffect(() => {
    if (!player) return;
    setGamesLoading(true);
    fetch(`/api/player/${player.id}/games`)
      .then(res => {
        if (!res.ok) throw new Error('Failed to fetch games');
        return res.json();
      })
      .then(data => {
        setRecentGames(data);
        setGamesLoading(false);
      })
      .catch(() => {
        setGamesError('Could not load recent games.');
        setGamesLoading(false);
      });
  }, [player]);

  const startEditingName = () => {
    setNewPlayerName(player?.name || '');
    setIsEditingName(true);
  };

  const savePlayerName = () => {
    if (!player || !newPlayerName.trim()) return;
    // TODO: Send update to backend (not implemented yet)
    const updatedPlayer = { ...player, name: newPlayerName.trim() };
    setPlayer(updatedPlayer);
    localStorage.setItem('player', JSON.stringify(updatedPlayer));
    setIsEditingName(false);
  };

  const cancelEditingName = () => {
    setIsEditingName(false);
  };

  const findMatch = () => {
    navigate('/matchmaking');
  };

  return (
    <div style={{ minHeight: '100vh', background: '#121213', color: 'white', fontFamily: `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif` }}>
      <div style={{ maxWidth: 600, margin: '0 auto', padding: '2rem', paddingTop: '1rem' }}>
        <div style={{ background: 'rgba(255,255,255,0.1)', padding: '1.5rem', borderRadius: 8, marginBottom: '2rem' }}>
          {!isEditingName ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '1.2rem' }}>
              <span>Player: {player?.name} {player && !player.registered && <span style={{ fontSize: '0.9rem', color: '#f3f3f3', marginLeft: 8 }}>(Guest)</span>}</span>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <button style={{ background: 'none', border: 'none', color: '#538d4e', padding: '0.5rem', borderRadius: 4, cursor: 'pointer' }} onClick={startEditingName} aria-label="Edit name">
                  <FaPencilAlt />
                </button>
              </div>
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
        </div>
        {/* Recent games */}
        <div style={{ background: 'rgba(255,255,255,0.1)', padding: '1.5rem', borderRadius: 8 }}>
          <h2 style={{ fontSize: '1.5rem', marginBottom: '1rem', color: '#818384' }}>Recent Games</h2>
          {gamesLoading ? (
            <div style={{ color: 'white', textAlign: 'center' }}>Loading games...</div>
          ) : gamesError ? (
            <div style={{ color: 'red', textAlign: 'center' }}>{gamesError}</div>
          ) : !player ? (
            <div style={{ color: 'white', textAlign: 'center' }}>Please log in to see your games.</div>
          ) : player && !player.registered ? (
            <div style={{ color: 'white', textAlign: 'center' }}>Guest players do not have recent games.</div>
          ) : !recentGames || recentGames.length === 0 ? (
            <div style={{ color: 'white', textAlign: 'center' }}>No recent games found.</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0 12px 8px 12px', borderBottom: '1px solid rgba(255,255,255,0.1)', marginBottom: 12 }}>
                <span style={{ color: '#818384', fontSize: '0.9rem', fontWeight: 500, minWidth: 100, flex: 1 }}>Opponent</span>
                <span style={{ color: '#818384', fontSize: '0.9rem', fontWeight: 500, minWidth: 80, textAlign: 'left', padding: '0 4rem' }}>Result</span>
                <span style={{ color: '#818384', fontSize: '0.9rem', fontWeight: 500, minWidth: 80, textAlign: 'right' }}>Date</span>
              </div>
              {recentGames.map(game => {
                // Determine opponent name
                const opponent = player && game.first_player === player.id ? game.second_player : game.first_player;
                // Format date
                const date = new Date(game.created_at).toLocaleDateString();
                // Result (placeholder logic)
                let result = 'In Progress';
                if (game.result === player?.id) result = 'Won';
                else if (game.result && game.result !== '') result = 'Lost';
                return (
                  <a
                    key={game.id}
                    href={`#/game/${game.id}`}
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
                      <span style={{ fontWeight: 'bold', minWidth: 100, flex: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{opponent}</span>
                      <span style={{
                        padding: '0.25rem 0.5rem',
                        borderRadius: 4,
                        fontSize: '0.9rem',
                        minWidth: 80,
                        textAlign: 'left',
                        display: 'inline-block',
                        margin: '0 4rem',
                        whiteSpace: 'nowrap',
                        color: result === 'Won' ? '#538d4e' : result === 'Lost' ? '#ff4d4d' : '#b59f3b',
                        fontWeight: 'bold',
                      }}>{result}</span>
                    </div>
                    <div style={{ color: '#818384', fontSize: '0.9rem', minWidth: 80, textAlign: 'right', whiteSpace: 'nowrap' }}>{date}</div>
                  </a>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default Home; 