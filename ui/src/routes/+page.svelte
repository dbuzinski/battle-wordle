<script>
  import { onMount } from 'svelte';
  import { browser } from '$app/environment';

  let playerName = '';
  let newPlayerName = '';
  let isEditingName = false;
  let recentGames = [];

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

  function getCookie(name) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop()?.split(';').shift() || null;
    return null;
  }

  function setCookie(name, value) {
    const expires = new Date();
    expires.setFullYear(expires.getFullYear() + 1);
    const cookieString = `${name}=${value};path=/;expires=${expires.toUTCString()};SameSite=None;Secure`;
    document.cookie = cookieString;
  }

  function generateUUID() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
      const r = Math.random() * 16 | 0;
      const v = c === 'x' ? r : (r & 0x3 | 0x8);
      return v.toString(16);
    });
  }

  // Load player name from cookie
  onMount(() => {
    if (browser) {
      playerName = getCookie('playerName') || generateRandomUsername();
      setCookie('playerName', playerName);
      loadRecentGames();
    }
  });

  function loadRecentGames() {
    const playerId = getCookie('playerId');
    console.log('Loading recent games for playerId:', playerId);
    
    if (!playerId) {
      console.log('No playerId found, generating new one');
      // Generate a new player ID if one doesn't exist
      const newPlayerId = generateUUID();
      setCookie('playerId', newPlayerId);
      
      // Save the player name with the new ID
      const serverUrl = window.location.protocol === 'https:' ? 'https:' : 'http:';
      fetch(`${serverUrl}//${window.location.hostname}:8080/api/set-player-name`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          playerId: newPlayerId,
          playerName
        })
      }).catch(error => {
        console.error('Error saving player name:', error);
      });

      // No need to load recent games for a new player
      return;
    }

    const serverUrl = window.location.protocol === 'https:' ? 'https:' : 'http:';
    console.log('Fetching recent games from:', `${serverUrl}//${window.location.hostname}:8080/api/recent-games?playerId=${playerId}`);
    
    fetch(`${serverUrl}//${window.location.hostname}:8080/api/recent-games?playerId=${playerId}`)
      .then(response => {
        console.log('Recent games response status:', response.status);
        return response.json();
      })
      .then(games => {
        console.log('Received games:', games);
        recentGames = games || [];
      })
      .catch(error => {
        console.error('Error loading recent games:', error);
        recentGames = [];
      });
  }

  function getGameStatus(game) {
    console.log('Getting game status for game:', game);
    if (!game) {
      console.log('Game is null/undefined');
      return '';
    }
    
    // If there's a loser, the game is finished
    if (game.loserId) {
      const status = game.loserId === getCookie('playerId') ? 'Lost' : 'Won';
      console.log('Game is finished, status:', status);
      return status;
    }
    
    // Game is in progress
    const currentPlayerId = getCookie('playerId');
    if (game.currentPlayer === currentPlayerId) {
      console.log('Game in progress, your turn');
      return 'Your Turn';
    } else {
      console.log('Game in progress, opponent\'s turn');
      return 'Opponent\'s Turn';
    }
  }

  function startEditingName() {
    newPlayerName = playerName;
    isEditingName = true;
  }

  async function savePlayerName() {
    if (newPlayerName.trim()) {
      playerName = newPlayerName.trim();
      setCookie('playerName', playerName);
      
      // Save name to database
      const playerId = getCookie('playerId');
      if (playerId) {
        try {
          const serverUrl = window.location.protocol === 'https:' ? 'https:' : 'http:';
          const response = await fetch(`${serverUrl}//${window.location.hostname}:8080/api/set-player-name`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              playerId,
              playerName
            })
          });
          
          if (!response.ok) {
            console.error('Failed to save player name');
          }
        } catch (error) {
          console.error('Error saving player name:', error);
        }
      }
      
      isEditingName = false;
    }
  }

  function cancelEditingName() {
    isEditingName = false;
  }

  function startNewGame() {
    window.location.href = '/games';
  }

  function findMatch() {
    window.location.href = '/matchmaking';
  }
</script>

<div class="container">
  <h1>Battle Wordle</h1>
  
  <div class="player-section">
    {#if !isEditingName}
      <div class="player-name">
        {#if playerName}
          <span>Player: {playerName}</span>
          <button class="edit-btn" on:click={startEditingName}>Edit</button>
        {:else}
          <button class="edit-btn" on:click={startEditingName}>Set Player Name</button>
        {/if}
      </div>
    {:else}
      <div class="name-edit">
        <input 
          type="text" 
          bind:value={newPlayerName} 
          placeholder="Enter your name"
          maxlength="20"
        />
        <div class="name-edit-buttons">
          <button class="save-btn" on:click={savePlayerName}>Save</button>
          <button class="cancel-btn" on:click={cancelEditingName}>Cancel</button>
        </div>
      </div>
    {/if}
  </div>

  <div class="game-actions">
    <button class="action-btn new-game" on:click={startNewGame}>
      New Game
    </button>
    <button class="action-btn find-match" on:click={findMatch}>
      Find Match
    </button>
  </div>

  {#if recentGames.length > 0}
    <div class="recent-games">
      <h2>Recent Games</h2>
      <div class="games-list">
        {#each recentGames as game}
          <div class="game-item">
            <div class="game-info">
              <span class="opponent">vs {game.opponentName}</span>
              <span class="result {getGameStatus(game).toLowerCase()}">{getGameStatus(game)}</span>
            </div>
            <div class="game-date">{game.date}</div>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  :global(body) { 
    background: #121213; 
    margin: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
  }

  .container {
    max-width: 600px;
    margin: 0 auto;
    padding: 2rem;
    color: white;
  }

  h1 {
    text-align: center;
    font-size: 2.5rem;
    margin-bottom: 2rem;
    background: linear-gradient(45deg, #538d4e, #b59f3b);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .player-section {
    background: rgba(255, 255, 255, 0.1);
    padding: 1.5rem;
    border-radius: 8px;
    margin-bottom: 2rem;
  }

  .player-name {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 1.2rem;
  }

  .edit-btn {
    background: none;
    border: 1px solid #538d4e;
    color: #538d4e;
    padding: 0.5rem 1rem;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .edit-btn:hover {
    background: #538d4e;
    color: white;
  }

  .name-edit {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .name-edit input {
    padding: 0.5rem;
    border: 1px solid #3a3a3c;
    border-radius: 4px;
    background: #1a1a1a;
    color: white;
    font-size: 1rem;
  }

  .name-edit-buttons {
    display: flex;
    gap: 1rem;
  }

  .save-btn, .cancel-btn {
    padding: 0.5rem 1rem;
    border-radius: 4px;
    cursor: pointer;
    border: none;
    font-weight: bold;
  }

  .save-btn {
    background: #538d4e;
    color: white;
  }

  .cancel-btn {
    background: #3a3a3c;
    color: white;
  }

  .game-actions {
    display: flex;
    gap: 1rem;
    margin-bottom: 2rem;
  }

  .action-btn {
    flex: 1;
    padding: 1rem;
    border: none;
    border-radius: 8px;
    font-size: 1.1rem;
    font-weight: bold;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .new-game {
    background: #538d4e;
    color: white;
  }

  .find-match {
    background: #b59f3b;
    color: white;
  }

  .action-btn:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 8px rgba(0, 0, 0, 0.2);
  }

  .recent-games {
    background: rgba(255, 255, 255, 0.1);
    padding: 1.5rem;
    border-radius: 8px;
  }

  h2 {
    font-size: 1.5rem;
    margin-bottom: 1rem;
    color: #818384;
  }

  .games-list {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .game-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    background: rgba(0, 0, 0, 0.2);
    border-radius: 4px;
  }

  .game-info {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .opponent {
    font-weight: bold;
  }

  .result {
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.9rem;
  }

  .result.won {
    background: #538d4e;
  }

  .result.lost {
    background: #b59f3b;
  }

  .result.draw {
    background: #3a3a3c;
  }

  .result.your.turn {
    background: #538d4e;
    animation: pulse 2s infinite;
  }

  .result.opponent.s.turn {
    background: #b59f3b;
  }

  @keyframes pulse {
    0% {
      opacity: 1;
    }
    50% {
      opacity: 0.7;
    }
    100% {
      opacity: 1;
    }
  }

  .game-date {
    color: #818384;
    font-size: 0.9rem;
  }
</style>
