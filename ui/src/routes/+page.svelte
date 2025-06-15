<script lang="ts">
  import { onMount } from 'svelte';
  import { browser } from '$app/environment';

  interface Game {
    id: string;
    date: string;
    loserId: string | null;
    currentPlayer: string;
    opponentName: string;
    opponentId: string;
    isInProgress: boolean;
    guesses: string[];
    solution?: string;
    gameOver?: boolean;
  }

  let playerName = '';
  let newPlayerName = '';
  let isEditingName = false;
  let recentGames: Game[] = [];
  let gameOver = false;
  let loserId = '';
  let solution = '';
  let rematchGameId = '';
  let socket: WebSocket | null = null;
  let gameId = '';
  let currentPage = 1;
  const gamesPerPage = 10;
  let totalPages = 1;

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

  function getCookie(name: string): string | null {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop()?.split(';').shift() || null;
    return null;
  }

  function setCookie(name: string, value: string): void {
    const expires = new Date();
    expires.setFullYear(expires.getFullYear() + 1);
    const cookieString = `${name}=${value};path=/;expires=${expires.toUTCString()};SameSite=None;Secure`;
    document.cookie = cookieString;
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
    
    if (!playerId) {
      // Generate a new player ID if one doesn't exist
      const newPlayerId = crypto.randomUUID();
      setCookie('playerId', newPlayerId);
      
      // Save the player name with the new ID
      fetch(`${import.meta.env.VITE_API_URL}/set-player-name`, {
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

      return;
    }

    fetch(`${import.meta.env.VITE_API_URL}/recent-games?playerId=${playerId}`)
      .then(response => response.json())
      .then(games => {
        recentGames = games || [];
        totalPages = Math.ceil(recentGames.length / gamesPerPage);
        currentPage = 1;
      })
      .catch(error => {
        console.error('Error loading recent games:', error);
        recentGames = [];
        totalPages = 1;
        currentPage = 1;
      });
  }

  function getPaginatedGames() {
    const startIndex = (currentPage - 1) * gamesPerPage;
    const endIndex = startIndex + gamesPerPage;
    return recentGames.slice(startIndex, endIndex);
  }

  function nextPage() {
    if (currentPage < totalPages) {
      currentPage++;
    }
  }

  function previousPage() {
    if (currentPage > 1) {
      currentPage--;
    }
  }

  function getGameStatus(game: Game): string {
    if (!game) return '';
    
    if (game.loserId) {
      return game.loserId === getCookie('playerId') ? 'Lost' : 'Won';
    }
    
    const currentPlayerId = getCookie('playerId');
    if (game.isInProgress) {
      return game.currentPlayer === currentPlayerId ? 'Your Turn' : 'Opponent\'s Turn';
    }
    
    return 'Draw';
  }

  function getGameStatusClass(game: Game): string {
    if (game.isInProgress) {
      return 'status-in-progress';
    }
    if (game.loserId === getCookie('playerId')) {
      return 'status-lost';
    }
    if (game.loserId) {
      return 'status-won';
    }
    return 'status-draw';
  }

  function getGameStatusText(game: Game): string {
    if (game.isInProgress) {
      if (game.currentPlayer === getCookie('playerId')) {
        return 'Your Turn';
      } else {
        return 'Opponent\'s Turn';
      }
    }
    if (game.loserId === getCookie('playerId')) {
      return 'Lost';
    }
    if (game.loserId) {
      return 'Won';
    }
    return 'Draw';
  }

  function startEditingName() {
    newPlayerName = playerName;
    isEditingName = true;
  }

  async function savePlayerName() {
    if (newPlayerName.trim()) {
      playerName = newPlayerName.trim();
      setCookie('playerName', playerName);
      
      const playerId = getCookie('playerId');
      if (playerId) {
        try {
          const response = await fetch(`${import.meta.env.VITE_API_URL}/set-player-name`, {
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

  function handleRematch() {
    const rematchGameId = getCookie('rematchGameId');
    if (rematchGameId) {
      window.location.href = `/games/${rematchGameId}`;
    }
  }

  function handleFindMatch() {
    window.location.href = '/matchmaking';
  }

  // Initialize socket and set up message handler
  onMount(() => {
    // Get game ID from URL
    const pathParts = window.location.pathname.split('/');
    gameId = pathParts[pathParts.length - 1];
    
    socket = new WebSocket(`${import.meta.env.VITE_WS_URL}?game=${gameId}`);
    
    socket.onmessage = (event: MessageEvent) => {
      const message = JSON.parse(event.data);
      if (message.type === 'game_over') {
        gameOver = true;
        loserId = message.loserId;
        solution = message.solution;
        rematchGameId = message.rematchGameId;
      }
    };
  });
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
    <button class="action-btn find-match" on:click={findMatch}>
      Find Match
    </button>
    <button class="action-btn new-game" on:click={startNewGame}>
      New Game
    </button>
  </div>

  {#if recentGames.length > 0}
    <div class="recent-games">
      <h2>Recent Games</h2>
      <div class="games-list">
        <div class="game-header">
          <div class="game-info">
            <span class="header-label opponent-header">Opponent</span>
            <span class="header-label result-header">Result</span>
          </div>
          <div class="header-label date-header">Date</div>
        </div>
        {#each getPaginatedGames() as game}
          <a href="/games?game={game.id}" class="game-item">
            <div class="game-info">
              <span class="opponent">{game.opponentName}</span>
              <span class="result {getGameStatusClass(game)}">{getGameStatusText(game)}</span>
            </div>
            <div class="game-date">{game.date}</div>
          </a>
        {/each}
      </div>
      {#if totalPages > 1}
        <div class="pagination">
          <button 
            class="page-btn" 
            on:click={previousPage} 
            disabled={currentPage === 1}
          >
            Previous
          </button>
          <span class="page-info">Page {currentPage} of {totalPages}</span>
          <button 
            class="page-btn" 
            on:click={nextPage} 
            disabled={currentPage === totalPages}
          >
            Next
          </button>
        </div>
      {/if}
    </div>
  {/if}

  {#if gameOver}
    <div class="game-over">
      <h2>Game Over!</h2>
      <p>
        {#if loserId === getCookie('playerId')}
          You lost! The word was {solution}.
        {:else if loserId}
          You won! The word was {solution}.
        {:else}
          It's a draw! The word was {solution}.
        {/if}
      </p>
      <div class="game-over-buttons">
        <button class="rematch" on:click={handleRematch}>Rematch</button>
        <button class="find-match" on:click={handleFindMatch}>Find Match</button>
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
    background: #538d4e;
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
    padding: 12px;
    margin-bottom: 8px;
    border-radius: 8px;
    background-color: rgba(255, 255, 255, 0.1);
    transition: all 0.2s ease;
    text-decoration: none;
    color: white;
  }

  .game-item:hover {
    transform: translateX(5px);
    background-color: rgba(255, 255, 255, 0.15);
  }

  .game-item.status-won {
    background-color: rgba(83, 141, 78, 0.2);
  }

  .game-item.status-won:hover {
    background-color: rgba(83, 141, 78, 0.3);
  }

  .game-item.status-lost {
    background-color: rgba(255, 77, 77, 0.2);
  }

  .game-item.status-lost:hover {
    background-color: rgba(255, 77, 77, 0.3);
  }

  .game-item.status-in-progress {
    background-color: rgba(181, 159, 59, 0.2);
  }

  .game-item.status-in-progress:hover {
    background-color: rgba(181, 159, 59, 0.3);
  }

  .game-item.status-draw {
    background-color: rgba(129, 131, 132, 0.2);
  }

  .game-item.status-draw:hover {
    background-color: rgba(129, 131, 132, 0.3);
  }

  .game-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 12px 8px 12px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    margin-bottom: 12px;
  }

  .header-label {
    color: #818384;
    font-size: 0.9rem;
    font-weight: 500;
  }

  .opponent-header {
    min-width: 100px;
    flex: 1;
  }

  .result-header {
    min-width: 80px;
    text-align: left;
    padding: 0 4rem;
  }

  .date-header {
    min-width: 80px;
    text-align: right;
  }

  .game-info {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex: 1;
    min-width: 0;
  }

  .opponent {
    font-weight: bold;
    min-width: 100px;
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .result {
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.9rem;
    min-width: 80px;
    text-align: left;
    display: inline-block;
    margin: 0 4rem;
    white-space: nowrap;
  }

  .game-date {
    color: #818384;
    font-size: 0.9rem;
    min-width: 80px;
    text-align: right;
    white-space: nowrap;
  }

  .status-won {
    color: #538d4e;
    font-weight: bold;
  }

  .status-lost {
    color: #ff4d4d;
    font-weight: bold;
  }

  .status-in-progress {
    color: #b59f3b;
    font-weight: bold;
  }

  .status-draw {
    color: #818384;
    font-weight: bold;
  }

  .game-over-buttons {
    display: flex;
    gap: 1rem;
    justify-content: center;
    margin-top: 1rem;
  }

  .rematch, .find-match {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    font-size: 1rem;
    cursor: pointer;
    background: #538d4e;
    color: white;
  }

  .rematch:hover, .find-match:hover {
    opacity: 0.9;
  }

  .pagination {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 1rem;
    margin-top: 1.5rem;
    padding-top: 1rem;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
  }

  .page-btn {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    background: #538d4e;
    color: white;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .page-btn:disabled {
    background: #3a3a3c;
    cursor: not-allowed;
    opacity: 0.5;
  }

  .page-btn:not(:disabled):hover {
    background: #4a7d45;
    transform: translateY(-1px);
  }

  .page-info {
    color: #818384;
    font-size: 0.9rem;
  }

  @media (max-width: 480px) {
    .game-header {
      display: none;
    }

    .game-item {
      flex-direction: column;
      align-items: flex-start;
      padding: 1rem;
    }

    .game-info {
      width: 100%;
      justify-content: space-between;
      gap: 0.5rem;
    }

    .opponent {
      font-weight: bold;
      font-size: 1rem;
    }

    .result {
      font-size: 0.9rem;
      padding: 0.25rem 0.75rem;
    }

    .game-date {
      width: 100%;
      text-align: left;
      margin-top: 0.5rem;
      font-size: 0.85rem;
      color: #a0a0a0;
    }
  }
</style>
