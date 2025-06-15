<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { browser } from '$app/environment';
    import { allowedGuesses } from '$lib/data/allowed_guesses';
  
    // Constants
    const WORD_LENGTH = 5;
    const MAX_GUESSES = 6;
    const KEYBOARD_ROWS = [
      ['q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p'],
      ['a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l'],
      ['enter', 'z', 'x', 'c', 'v', 'b', 'n', 'm', 'backspace']
    ];
  
    // Game state
    let gameId = '';
    let socket = null;
    let playerId = '';
    let showMessage = false;
    let message = '';
    let turnStatusMessage = '';
    let messageTimeout = null;
    let isMyTurn = false;
    let isGameOver = false;
    let isSpectator = false;
    let allGuesses = [];
    let statuses = [];
    let solution = '';
    let guesses = Array(MAX_GUESSES).fill(null).map(() => Array(WORD_LENGTH).fill(''));
    let currentGuessIndex = 0;
    let currentLetterIndex = 0;
    let letterStatuses = {};
    let playerIds = [];
    let isInvalidGuess = false;
    let invalidGuessTimeout = null;
    let isMenuOpen = false;
    let rematchGameId = '';
    let isInQueue = false;
    let queueSocket = null;
    let playerStats = {};
    let playerNames = {};
  
    // Cookie management
    function getCookie(name) {
      console.log(`[getCookie] Looking for cookie "${name}"`);
      const value = `; ${document.cookie}`;
      const parts = value.split(`; ${name}=`);
      if (parts.length === 2) {
        const cookieValue = parts.pop()?.split(';').shift();
        console.log(`[getCookie] Found cookie "${name}" with value:`, cookieValue);
        return cookieValue;
      }
      console.log(`[getCookie] Cookie "${name}" not found`);
      return null;
    }
  
    function setCookie(name, value) {
      const expires = new Date();
      expires.setFullYear(expires.getFullYear() + 1);
      const cookieString = `${name}=${value};path=/;expires=${expires.toUTCString()};SameSite=None;Secure`;
      console.log(`[setCookie] Setting cookie:`, cookieString);
      document.cookie = cookieString;
  
      // Check immediately if cookie was set
      const checkValue = getCookie(name);
      if (checkValue === value) {
        console.log(`[setCookie] Cookie "${name}" successfully set.`);
      } else {
        console.warn(`[setCookie] Failed to set cookie "${name}". Current value:`, checkValue);
      }
    }
  
    function getPlayerId() {
      console.log('[getPlayerId] Start');
      const existingId = getCookie('playerId');
      if (existingId) {
        console.log('[getPlayerId] Returning existing playerId:', existingId);
        return existingId;
      }
  
      const newId = crypto.randomUUID();
      console.log('[getPlayerId] No playerId found, generating new one:', newId);
      setCookie('playerId', newId);
  
      // Confirm it's set
      const confirmId = getCookie('playerId');
      if (confirmId === newId) {
        console.log('[getPlayerId] New playerId successfully stored in cookie:', confirmId);
      } else {
        console.warn('[getPlayerId] Failed to store new playerId in cookie. Found:', confirmId);
      }
  
      return newId;
    }
  
    // UI state
    function showMessageTemporarily(msg) {
      message = msg;
      showMessage = true;
      if (messageTimeout) clearTimeout(messageTimeout);
      messageTimeout = window.setTimeout(() => {
        showMessage = false;
      }, 3000);
    }
  
    function showInvalidGuess() {
      isInvalidGuess = true;
      if (invalidGuessTimeout) clearTimeout(invalidGuessTimeout);
      invalidGuessTimeout = window.setTimeout(() => {
        isInvalidGuess = false;
      }, 600);
    }
  
    function createNewGame() {
      gameId = crypto.randomUUID();
      window.history.pushState({}, '', `?game=${gameId}`);
      initializeWebSocket();
    }
  
    function resetGameState() {
      isGameOver = false;
      isMyTurn = false;
      solution = '';
      allGuesses = [];
      guesses = Array(MAX_GUESSES).fill(null).map(() => Array(WORD_LENGTH).fill(''));
      statuses = [];
      currentGuessIndex = 0;
      currentLetterIndex = 0;
      letterStatuses = {};
      showMessage = false;
      message = '';
    }
  
    function startNewGame() {
      resetGameState();
      createNewGame();
    }
  
    function initializeWebSocket() {
      const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${wsProtocol}//${window.location.hostname}:8080/ws?game=${gameId}`;
      
      socket = new WebSocket(wsUrl);
      
      socket.onopen = () => {
        console.log('WebSocket connected');
        socket.send(JSON.stringify({
          type: 'join',
          from: playerId
        }));
      };
  
      socket.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        
        if (msg.type === 'game_state') {
          handleGameState(msg);
        } else if (msg.type === 'game_over') {
          handleGameOver(msg);
        }
      };
  
      socket.onerror = (error) => {
        console.error('WebSocket error:', error);
      };
  
      socket.onclose = () => {
        console.log('WebSocket connection closed');
      };
    }
  
    function handleGameState(msg) {
      console.log('[handleGameState] Received game state:', msg);
      playerIds = msg.players || [];
      playerNames = msg.playerNames || {};
      console.log('[handleGameState] Player IDs:', playerIds);
      console.log('[handleGameState] Player Names:', playerNames);
      console.log('[handleGameState] Current player ID:', playerId);
      console.log('[handleGameState] Opponent ID:', playerIds.find(id => id !== playerId));
      console.log('[handleGameState] Opponent Name:', playerNames[playerIds.find(id => id !== playerId)]);
      
      isSpectator = !playerIds.includes(playerId);
      isMyTurn = !isSpectator && msg.currentPlayer === playerId;
      
      if (msg.solution) {
        solution = msg.solution;
      }
      
      if (msg.guesses) {
        allGuesses = msg.guesses;
        updateBoard();
      } else {
        initializeBoard();
      }
      
      // Store rematch game ID if available
      if (msg.rematchGameId) {
        rematchGameId = msg.rematchGameId;
        console.log('[handleGameState] Set rematch game ID:', rematchGameId);
      }
      
      // Show game over screen if the game is over
      if (msg.gameOver) {
        handleGameOver(msg);
      } else {
        showMessage = false;
        message = '';
      }
      
      updateTurnStatus(msg);
  
      // Fetch player stats when game starts
      if (playerIds.length === 2 && !isSpectator) {
        fetchPlayerStats();
      }
    }
  
    async function fetchPlayerStats() {
      try {
        const serverUrl = window.location.protocol === 'https:' ? 'https:' : 'http:';
        const opponentId = playerIds.find(id => id !== playerId);
        if (opponentId) {
          const response = await fetch(`${serverUrl}//${window.location.hostname}:8080/api/head-to-head-stats?playerId=${playerId}&opponentId=${opponentId}`);
          if (response.ok) {
            const stats = await response.json();
            playerStats[playerId] = stats;
          }
        }
      } catch (error) {
        console.error('Error fetching player stats:', error);
      }
    }
  
    function handleGameOver(msg) {
      console.log('[handleGameOver] Received game over message:', msg);
      isGameOver = true;
      isMyTurn = false;
      solution = msg.solution;
      allGuesses = msg.guesses || [];
      rematchGameId = msg.rematchGameId;
      console.log('[handleGameOver] Set rematch game ID:', rematchGameId);
      
      // Update player info
      playerIds = msg.players || [];
      playerNames = msg.playerNames || {};
      isSpectator = !playerIds.includes(playerId);
      
      // Update the board with the final state
      updateBoard();
      
      // Fetch updated stats after game over
      if (msg.players.includes(playerId)) {
        fetchPlayerStats();
      }
      
      const gameOverMessage = getGameOverMessage(msg);
      message = gameOverMessage;
      showMessage = true;
      turnStatusMessage = '';
    }
  
    function getGameOverMessage(msg) {
      if (!msg.players.includes(playerId)) {
        return `Game Over!`;
      }
      
      if (msg.loserId === playerId) {
        return `You lost!`;
      }
      
      if (msg.loserId) {
        return `You won!`;
      }
      
      return `Draw!`;
    }
  
    function updateTurnStatus(msg) {
      if (isGameOver) {
        turnStatusMessage = '';
      } else if (isSpectator) {
        turnStatusMessage = "You are spectating this game";
      } else if (msg.currentPlayer === 'waiting_for_opponent') {
        turnStatusMessage = "Waiting for opponent to join...";
        isMyTurn = false;
      } else {
        turnStatusMessage = isMyTurn ? "Your turn!" : "Waiting for opponent...";
      }
    }
  
    function startRematch() {
      console.log('[startRematch] Attempting to start rematch with ID:', rematchGameId);
      if (!rematchGameId) {
        console.error('[startRematch] No rematch game ID available');
        return;
      }
      
      const previousGameId = gameId;
      gameId = rematchGameId;
      console.log('[startRematch] Switching from game', previousGameId, 'to rematch game', gameId);
      
      // Update URL
      const url = new URL(window.location.href);
      url.searchParams.set('game', gameId);
      url.searchParams.set('rematch', 'true');
      url.searchParams.set('previousGame', previousGameId);
      window.history.pushState({}, '', url);
      
      // Reset game state
      resetGameState();
      
      // Connect to rematch game
      const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${wsProtocol}//${window.location.hostname}:8080/ws?game=${gameId}&rematch=true&previousGame=${previousGameId}`;
      console.log('[startRematch] Connecting to WebSocket:', wsUrl);
      
      socket = new WebSocket(wsUrl);
      
      socket.onopen = () => {
        console.log('WebSocket connected for rematch');
        socket.send(JSON.stringify({
          type: 'join',
          from: playerId
        }));
      };
  
      socket.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        
        if (msg.type === 'game_state') {
          handleGameState(msg);
        } else if (msg.type === 'game_over') {
          handleGameOver(msg);
        }
      };
  
      socket.onerror = (error) => {
        console.error('WebSocket error:', error);
      };
  
      socket.onclose = () => {
        console.log('WebSocket connection closed');
      };
    }
  
    // Game logic
    function getGuessStatuses(guess, solution) {
      if (!solution) return Array(guess.length).fill('absent');
      
      const statuses = Array(guess.length).fill('absent');
      const solutionChars = solution.split('');
      const taken = Array(guess.length).fill(false);
  
      // First pass: mark correct letters
      for (let i = 0; i < guess.length; i++) {
        if (guess[i].toUpperCase() === solution[i]) {
          statuses[i] = 'correct';
          taken[i] = true;
        }
      }
  
      // Second pass: mark present letters
      for (let i = 0; i < guess.length; i++) {
        if (statuses[i] === 'correct') continue;
        const idx = solutionChars.findIndex((c, j) => c === guess[i].toUpperCase() && !taken[j]);
        if (idx !== -1) {
          statuses[i] = 'present';
          taken[idx] = true;
        }
      }
  
      return statuses;
    }
  
    function updateBoard() {
      guesses = Array(MAX_GUESSES).fill(null).map(() => Array(WORD_LENGTH).fill(''));
      statuses = [];
      letterStatuses = {};
      
      allGuesses.forEach((guess, index) => {
        if (index < MAX_GUESSES) {
          guesses[index] = guess.split('');
          const guessStatuses = getGuessStatuses(guess, solution);
          statuses[index] = guessStatuses;
        }
      });
      
      currentGuessIndex = allGuesses.length;
      currentLetterIndex = 0;
  
      setTimeout(() => {
        allGuesses.forEach((guess, index) => {
          if (index < MAX_GUESSES) {
            const guessStatuses = getGuessStatuses(guess, solution);
            guessStatuses.forEach((status, i) => {
              const letter = guess[i].toUpperCase();
              const currentStatus = letterStatuses[letter] || null;
              letterStatuses[letter] = mergeStatus(currentStatus, status);
            });
          }
        });
      }, 600);
    }
  
    function initializeBoard() {
      updateBoard();
    }
  
    function validateGuess(guess) {
      const guessArray = guess.split('');
  
      if (!allowedGuesses.includes(guess)) {
        showMessageTemporarily("Not in word list");
        showInvalidGuess();
        return false;
      }
      
      if (!validateCorrectLetters(guessArray)) return false;
      if (!validatePresentLetters(guessArray)) return false;
      if (!validateRequiredLetters(guessArray)) return false;
      if (!validateAbsentLetters(guessArray)) return false;
  
      return true;
    }
  
    function validateCorrectLetters(guessArray) {
      for (let prevGuessIndex = 0; prevGuessIndex < allGuesses.length; prevGuessIndex++) {
        const prevGuess = allGuesses[prevGuessIndex];
        const prevStatuses = statuses[prevGuessIndex];
        
        for (let i = 0; i < prevGuess.length; i++) {
          if (prevStatuses[i] === 'correct') {
            const correctLetter = prevGuess[i].toUpperCase();
            if (guessArray[i].toUpperCase() !== correctLetter) {
              showMessageTemporarily(`Letter ${correctLetter} must be in position ${i + 1}`);
              showInvalidGuess();
              return false;
            }
          }
        }
      }
      return true;
    }
  
    function validatePresentLetters(guessArray) {
      for (let i = 0; i < guessArray.length; i++) {
        const letter = guessArray[i].toUpperCase();
        
        for (let prevGuessIndex = 0; prevGuessIndex < allGuesses.length; prevGuessIndex++) {
          const prevGuess = allGuesses[prevGuessIndex];
          const prevStatuses = statuses[prevGuessIndex];
          
          if (prevGuess[i].toUpperCase() === letter && prevStatuses[i] === 'present') {
            showMessageTemporarily(`Cannot use letter ${letter} in position ${i + 1}`);
            showInvalidGuess();
            return false;
          }
        }
      }
      return true;
    }
  
    function validateRequiredLetters(guessArray) {
      const requiredLetters = new Set();
      for (let prevGuessIndex = 0; prevGuessIndex < allGuesses.length; prevGuessIndex++) {
        const prevGuess = allGuesses[prevGuessIndex];
        const prevStatuses = statuses[prevGuessIndex];
        
        for (let i = 0; i < prevGuess.length; i++) {
          if (prevStatuses[i] === 'correct' || prevStatuses[i] === 'present') {
            requiredLetters.add(prevGuess[i].toUpperCase());
          }
        }
      }
      
      const usedLetters = new Set(guessArray.map(l => l.toUpperCase()));
      for (const requiredLetter of requiredLetters) {
        if (!usedLetters.has(requiredLetter)) {
          showMessageTemporarily(`Must use letter ${requiredLetter}`);
          showInvalidGuess();
          return false;
        }
      }
      return true;
    }
  
    function validateAbsentLetters(guessArray) {
      for (let i = 0; i < guessArray.length; i++) {
        const letter = guessArray[i].toUpperCase();
        if (letterStatuses[letter] === 'absent') {
          showMessageTemporarily(`Cannot use letter ${letter}`);
          showInvalidGuess();
          return false;
        }
      }
      return true;
    }
  
    function mergeStatus(current, newStatus) {
      if (!current) return newStatus;
      if (current === 'correct') return 'correct';
      if (current === 'present' && newStatus === 'correct') return 'correct';
      if (current === 'absent' && (newStatus === 'present' || newStatus === 'correct')) return newStatus;
      return current;
    }
  
    // Input handling
    function handleKey(e) {
      if (isGameOver || isSpectator || !isMyTurn || isMenuOpen) return;
  
      const key = e.key.toLowerCase();
  
      if (key === 'enter') {
        e.preventDefault();
        handleEnterKey();
      } else if (key === 'backspace') {
        handleBackspaceKey();
      } else if (/^[a-z]$/.test(key)) {
        handleLetterKey(key);
      }
    }
  
    function handleEnterKey() {
      const guess = guesses[currentGuessIndex].join('');
      if (guess.length === WORD_LENGTH) {
        if (!validateGuess(guess)) return;
        
        socket.send(JSON.stringify({
          type: 'guess',
          from: playerId,
          guess: guess
        }));
        
        guesses[currentGuessIndex] = Array(WORD_LENGTH).fill('');
        currentLetterIndex = 0;
      }
    }
  
    function handleBackspaceKey() {
      if (currentLetterIndex > 0) {
        currentLetterIndex--;
        guesses[currentGuessIndex][currentLetterIndex] = '';
      }
    }
  
    function handleLetterKey(key) {
      if (currentLetterIndex < WORD_LENGTH) {
        guesses[currentGuessIndex][currentLetterIndex] = key;
        currentLetterIndex++;
      }
    }
  
    function handleKeyPress(key) {
      if (isMenuOpen) return;
      handleKey({ key, preventDefault: () => {} });
    }
  
    // UI helpers
    function getTileColor(row, col) {
      const status = statuses[row]?.[col];
      if (status === 'correct') return '#538d4e';
      if (status === 'present') return '#b59f3b';
      if (status === 'absent') return '#3a3a3c';
      return '#121213';
    }
  
    function toggleMenu() {
      isMenuOpen = !isMenuOpen;
    }
  
    function closeMenu() {
      isMenuOpen = false;
    }
  
    async function findMatch() {
      if (isInQueue) return;
      
      // Reset game state before starting matchmaking
      resetGameState();
      isInQueue = true;
      closeMenu();
      
      // Get player name from cookie
      const playerName = getCookie('playerName');
      if (playerName) {
        try {
          // Save name to database before entering queue
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
            console.error('Failed to save player name before matchmaking');
            isInQueue = false;
            return;
          }
          
          // Wait a moment to ensure the name is saved in the database
          await new Promise(resolve => setTimeout(resolve, 100));
        } catch (error) {
          console.error('Error saving player name before matchmaking:', error);
          isInQueue = false;
          return;
        }
      }
      
      // Connect to queue WebSocket
      const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${wsProtocol}//${window.location.hostname}:8080/ws`;
      
      queueSocket = new WebSocket(wsUrl);
      
      queueSocket.onopen = () => {
        console.log('Queue WebSocket connected');
        queueSocket.send(JSON.stringify({
          type: 'queue',
          from: playerId
        }));
      };
  
      queueSocket.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        console.log('Queue message received:', msg);
        
        if (msg.type === 'match_found') {
          console.log('Match found with player names:', msg.playerNames);
          isInQueue = false;
          gameId = msg.gameId;
          
          // Update URL
          const url = new URL(window.location.href);
          url.searchParams.set('game', gameId);
          window.history.pushState({}, '', url);
          
          // Close queue socket
          queueSocket.close();
          queueSocket = null;
          
          // Initialize game socket
          initializeWebSocket();
        }
      };
  
      queueSocket.onerror = (error) => {
        console.error('Queue WebSocket error:', error);
        isInQueue = false;
        queueSocket = null;
      };
  
      queueSocket.onclose = () => {
        console.log('Queue WebSocket connection closed');
        isInQueue = false;
        queueSocket = null;
      };
    }
  
    // Initialize game
    onMount(() => {
      if (browser) {
        playerId = getPlayerId();
        
        const urlGameId = $page.url.searchParams.get('game');
        if (urlGameId) {
          gameId = urlGameId;
          initializeWebSocket();
        } else {
          createNewGame();
        }
      }
    });
  </script>
  
  <svelte:window on:keydown={handleKey} />
  
  <div class="container">
    <button 
      class="menu-icon" 
      on:click={toggleMenu}
      on:keydown={e => e.key === 'Enter' && toggleMenu()}
      aria-label="Toggle menu"
      aria-expanded={isMenuOpen}
      aria-controls="menu-sidebar"
    >
      <div class="hamburger" class:open={isMenuOpen}></div>
    </button>
  
    <div 
      class="menu-overlay" 
      class:open={isMenuOpen} 
      on:click={closeMenu}
      on:keydown={e => e.key === 'Escape' && closeMenu()}
      role="presentation"
      aria-hidden={!isMenuOpen}
    ></div>
    
    <div 
      class="menu-sidebar" 
      class:open={isMenuOpen}
      id="menu-sidebar"
      role="navigation"
      aria-label="Game menu"
    >
      <div class="menu-content">
        <div class="menu-header">
          <h3>Menu</h3>
        </div>
        <nav class="menu-nav">
          <a 
            href="/"
            class="menu-link"
            on:click={closeMenu}
          >
            <span class="menu-text">Home</span>
          </a>
          <button 
            class="menu-link" 
            on:click={findMatch} 
            class:disabled={isInQueue}
            type="button"
            aria-label="Find match"
            disabled={isInQueue}
          >
            <span class="menu-text">{isInQueue ? 'Finding Match...' : 'Find Match'}</span>
          </button>
          <button 
            class="menu-link" 
            on:click={startNewGame}
            type="button"
            aria-label="Start new game"
          >
            <span class="menu-text">New Game</span>
          </button>
        </nav>
      </div>
    </div>
  
    {#if isInQueue}
      <div class="queue-status">
        <div class="queue-spinner"></div>
        <p>Finding match...</p>
      </div>
    {/if}
  
    {#if playerIds.length === 2 && !isSpectator}
      <div class="score-display">
        <div class="score-item">
          {#if playerNames[playerIds.find(id => id !== playerId)]}
            <span class="score-label">vs {playerNames[playerIds.find(id => id !== playerId)]}</span>
          {:else}
            <span class="score-label">vs Player</span>
          {/if}
          {#if playerStats[playerId]}
            <span class="stats-text">H2H: W: {playerStats[playerId].wins} L: {playerStats[playerId].losses} D: {playerStats[playerId].draws}</span>
          {/if}
        </div>
      </div>
    {/if}
  
    <h1 style="text-align: center; color: white;">Battle Wordle</h1>
    <h2 style="text-align: center; color: white; font-size: 1.2em;">Try to avoid guessing the word!</h2>
  
    {#if turnStatusMessage}
      <div class="turn-status" class:spectator={isSpectator}>
        {turnStatusMessage}
      </div>
    {/if}
  
    {#if showMessage && !isGameOver}
      <div class="message" class:error={message.includes('error')} class:success={message.includes('won')}>
        {message}
      </div>
    {/if}
  
    {#if isGameOver}
      <div class="game-over">
        <p>{message}</p>
        <p>"{solution}"</p>
        {#if !isSpectator}
          <div class="game-over-buttons">
            <button class="game-over-btn rematch" on:click={startRematch}>
              Rematch
            </button>
            <button class="game-over-btn new-game" on:click={startNewGame}>
              New Game
            </button>
          </div>
        {/if}
      </div>
    {/if}
  
    <div class="grid" style="display: grid; justify-content: center; grid-template-columns: repeat(5, 50px); gap: 5px;">
      {#each guesses as guess, i}
        {#each guess as letter, j}
          <div
            class="tile"
            class:flipped={statuses[i]?.[j]}
            class:current-row={i === currentGuessIndex}
            class:shake={isInvalidGuess && i === currentGuessIndex}
            style="--tile-color: {getTileColor(i, j)}; --row-index: {i}; --col-index: {j}"
          >
            {letter.toUpperCase()}
          </div>
        {/each}
      {/each}
    </div>
  
    <div class="keyboard">
      {#each KEYBOARD_ROWS as row}
        <div class="kb-row">
          {#each row as key}
            {#if key === 'enter' || key === 'backspace'}
              <button 
                class="kb-key" 
                data-key={key}
                on:click={() => handleKeyPress(key)}
              >
                {key === 'enter' ? 'Enter' : '⌫'}
              </button>
            {:else}
              <button 
                class="kb-key" 
                data-key={key}
                class:correct={letterStatuses[key.toUpperCase()] === 'correct'}
                class:present={letterStatuses[key.toUpperCase()] === 'present'}
                class:absent={letterStatuses[key.toUpperCase()] === 'absent'}
                on:click={() => handleKeyPress(key)}
              >
                {key.toUpperCase()}
              </button>
            {/if}
          {/each}
        </div>
      {/each}
    </div>
  </div>
  
  <style>
    :global(body) { 
      background: #121213; 
      margin: 0;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
    }
    
    .container {
      max-width: 500px;
      margin: 0 auto;
      padding: 20px;
    }
  
    .tile {
      width: clamp(40px, 10vw, 50px);
      height: clamp(40px, 10vw, 50px);
      display: flex;
      justify-content: center;
      align-items: center;
      font-weight: bold;
      font-size: clamp(1.2rem, 4vw, 1.5rem);
      color: white;
      border: 2px solid #3a3a3c;
      background-color: #121213;
      transform-style: preserve-3d;
      transition: transform 0.6s cubic-bezier(0.4, 0, 0.2, 1), border-color 0.2s ease;
      position: relative;
      border-radius: 4px;
      box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
    }
  
    .tile:hover {
      border-color: #565758;
    }
  
    .tile::before {
      content: '';
      position: absolute;
      inset: 0;
      background-color: var(--tile-color, #121213);
      transform: rotateX(180deg);
      backface-visibility: hidden;
      border-radius: 2px;
    }
  
    .tile.flipped {
      transform: rotateX(360deg);
      background-color: var(--tile-color, #121213);
      border-color: var(--tile-color, #121213);
    }
  
    /* Add cascading delays for each tile in a row */
    .tile.flipped[style*="--col-index: 0"] { transition-delay: 0s; }
    .tile.flipped[style*="--col-index: 1"] { transition-delay: 0.1s; }
    .tile.flipped[style*="--col-index: 2"] { transition-delay: 0.2s; }
    .tile.flipped[style*="--col-index: 3"] { transition-delay: 0.3s; }
    .tile.flipped[style*="--col-index: 4"] { transition-delay: 0.4s; }
  
    /* Add a subtle bounce effect for the current row */
    .tile.current-row {
      animation: bounceTile 0.2s ease-in-out;
    }
  
    @keyframes bounceTile {
      0%, 100% {
        transform: scale(1);
      }
      50% {
        transform: scale(1.05);
      }
    }
  
    /* Add a subtle glow effect for the current row */
    .tile.current-row {
      box-shadow: 0 0 10px rgba(255, 255, 255, 0.1);
    }
  
    /* Add a subtle pulse animation for empty tiles in the current row */
    .tile.current-row:empty {
      animation: pulseTile 2s infinite;
    }
  
    @keyframes pulseTile {
      0%, 100% {
        border-color: #3a3a3c;
      }
      50% {
        border-color: #565758;
      }
    }
  
    .keyboard {
      margin-top: 2rem;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 8px;
      width: 100%;
      max-width: 500px;
      margin-left: auto;
      margin-right: auto;
      padding: 0 8px;
    }
  
    .kb-row { 
      display: flex; 
      gap: 6px;
      justify-content: center;
      width: 100%;
    }
  
    .kb-key {
      padding: 0;
      min-width: 0;
      border: none;
      border-radius: 4px;
      font-weight: bold;
      color: white;
      text-transform: uppercase;
      cursor: pointer;
      transition: background-color 0.2s ease;
      background-color: #818384;
      user-select: none;
      flex: 1;
      max-width: 43px;
      height: 58px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 0.875rem;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
    }
  
    /* Special keys (Enter and Backspace) */
    .kb-key[data-key="enter"],
    .kb-key[data-key="backspace"] {
      flex: 1.5;
      max-width: 65px;
      font-size: 0.75rem;
    }
  
    /* Media queries for different screen sizes */
    @media (max-width: 500px) {
      .keyboard {
        gap: 6px;
        padding: 0 6px;
      }
  
      .kb-row {
        gap: 4px;
      }
  
      .kb-key {
        height: 50px;
        font-size: 0.75rem;
      }
  
      .kb-key[data-key="enter"],
      .kb-key[data-key="backspace"] {
        max-width: 55px;
        font-size: 0.65rem;
      }
    }
  
    @media (max-width: 380px) {
      .keyboard {
        gap: 4px;
        padding: 0 4px;
      }
  
      .kb-row {
        gap: 3px;
      }
  
      .kb-key {
        height: 45px;
        font-size: 0.7rem;
      }
  
      .kb-key[data-key="enter"],
      .kb-key[data-key="backspace"] {
        max-width: 45px;
        font-size: 0.6rem;
      }
    }
  
    .kb-key:hover {
      opacity: 0.9;
    }
  
    .kb-key:active { 
      opacity: 0.8;
    }
  
    .kb-key.correct {
      background-color: #538d4e;
    }
  
    .kb-key.present {
      background-color: #b59f3b;
    }
  
    .kb-key.absent {
      background-color: #3a3a3c;
    }
  
    .message {
      position: fixed;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      padding: 12px 24px;
      border-radius: 8px;
      background-color: rgba(255, 255, 255, 0.95);
      color: #121213;
      z-index: 1000;
      text-align: center;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
      animation: slideDown 0.3s ease-out;
      font-size: clamp(0.9rem, 2.5vw, 1.1rem);
    }
  
    @keyframes slideDown {
      from {
        transform: translate(-50%, -60%);
        opacity: 0;
      }
      to {
        transform: translate(-50%, -50%);
        opacity: 1;
      }
    }
  
    .message.error {
      background-color: rgba(255, 68, 68, 0.95);
      color: white;
    }
  
    .message.success {
      background-color: rgba(68, 255, 68, 0.95);
      color: #121213;
    }
  
    .turn-status {
      text-align: center;
      color: white;
      font-size: clamp(1rem, 3vw, 1.2em);
      margin: 1rem 0;
      padding: 0.75rem;
      background-color: rgba(255, 255, 255, 0.1);
      border-radius: 8px;
      box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
      transition: all 0.3s ease;
    }
  
    .turn-status.spectator {
      background-color: rgba(255, 255, 255, 0.15);
      font-style: italic;
    }
  
    .game-over {
      position: fixed;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      padding: 24px;
      border-radius: 8px;
      background-color: rgba(255, 255, 255, 0.95);
      color: #121213;
      z-index: 1000;
      text-align: center;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
      animation: slideDown 0.3s ease-out;
      backdrop-filter: blur(8px);
      min-width: 280px;
    }
  
    .game-over p {
      margin: 0;
      font-size: clamp(0.9rem, 2.5vw, 1.1rem);
      line-height: 1.5;
    }
  
    .game-over p:last-of-type {
      margin-top: 8px;
      font-weight: bold;
    }
  
    .game-over-buttons {
      display: flex;
      gap: 12px;
      margin-top: 20px;
      justify-content: center;
    }
  
    .game-over-btn {
      padding: 12px 24px;
      border: none;
      border-radius: 8px;
      cursor: pointer;
      font-size: 1rem;
      font-weight: bold;
      transition: all 0.3s ease;
      min-width: 120px;
      background-color: #538d4e;
      color: white;
    }
  
    .game-over-btn:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 8px rgba(0, 0, 0, 0.2);
      background-color: #4a7d45;
    }
  
    .game-over-btn:active {
      transform: translateY(0);
      box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
    }
  
    h1 {
      font-size: clamp(1.8rem, 5vw, 2.5rem);
      margin-bottom: 0.5rem;
      background: linear-gradient(45deg, #538d4e, #b59f3b);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
    }
  
    h2 {
      font-size: clamp(1rem, 3vw, 1.2em);
      color: #818384;
      margin-top: 0;
    }
  
    .grid {
      display: grid;
      justify-content: center;
      grid-template-columns: repeat(5, 50px);
      gap: 5px;
      margin: 2rem 0;
      perspective: 1000px;
    }
  
    /* Add shake animation for invalid guesses */
    @keyframes shake {
      0%, 100% {
        transform: translateX(0);
      }
      10%, 30%, 50%, 70%, 90% {
        transform: translateX(-4px);
      }
      20%, 40%, 60%, 80% {
        transform: translateX(4px);
      }
    }
  
    .tile.shake {
      animation: shake 0.6s cubic-bezier(.36,.07,.19,.97) both;
    }
  
    .tile.shake.flipped {
      animation: shake 0.6s cubic-bezier(.36,.07,.19,.97) both, flipTile 0.6s cubic-bezier(0.4, 0, 0.2, 1) forwards;
    }
  
    .menu-icon {
      position: fixed;
      top: 20px;
      left: 20px;
      width: 30px;
      height: 30px;
      cursor: pointer;
      z-index: 1001;
      background: none;
      border: none;
      padding: 0;
      outline: none;
    }
  
    .hamburger {
      position: relative;
      width: 100%;
      height: 100%;
    }
  
    .hamburger::before,
    .hamburger::after {
      content: '';
      position: absolute;
      width: 100%;
      height: 2px;
      background-color: white;
      transition: all 0.3s ease;
      left: 0;
    }
  
    .hamburger::before {
      top: 25%;
    }
  
    .hamburger::after {
      bottom: 25%;
    }
  
    .hamburger.open::before {
      top: 50%;
      transform: translateY(-50%) rotate(45deg);
    }
  
    .hamburger.open::after {
      top: 50%;
      transform: translateY(-50%) rotate(-45deg);
    }
  
    .menu-overlay {
      position: fixed;
      top: 0;
      left: 0;
      width: 100%;
      height: 100%;
      background-color: rgba(0, 0, 0, 0.5);
      opacity: 0;
      visibility: hidden;
      transition: all 0.3s ease;
      z-index: 999;
    }
  
    .menu-overlay.open {
      opacity: 1;
      visibility: visible;
    }
  
    .menu-sidebar {
      position: fixed;
      top: 0;
      left: -300px;
      width: 300px;
      height: 100%;
      background-color: #1a1a1a;
      box-shadow: 2px 0 5px rgba(0, 0, 0, 0.2);
      transition: all 0.3s ease;
      z-index: 1000;
      border-right: 1px solid rgba(255, 255, 255, 0.1);
    }
  
    .menu-sidebar.open {
      left: 0;
    }
  
    .menu-content {
      padding: 20px;
      height: 100%;
      display: flex;
      flex-direction: column;
    }
  
    .menu-header {
      margin-top: 25px;
      margin-bottom: 30px;
      padding-bottom: 15px;
    }
  
    .menu-header h3 {
      display: none;
    }
  
    .menu-nav {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
  
    .menu-link {
      display: flex;
      align-items: center;
      padding: 16px;
      color: white;
      text-decoration: none;
      border-radius: 8px;
      transition: all 0.2s ease;
      cursor: pointer;
      font-size: 1.1rem;
      letter-spacing: 0.3px;
      background: none;
      border: none;
      width: 100%;
      text-align: left;
    }
  
    .menu-link:hover {
      transform: translateX(5px);
      color: #538d4e;
      background: none;
    }
  
    .menu-link:active {
      transform: translateX(0);
      background: none;
    }
  
    .menu-link.disabled {
      opacity: 0.5;
      cursor: not-allowed;
      transform: none !important;
      color: rgba(255, 255, 255, 0.5);
      background: none;
    }
  
    .menu-link.disabled:hover {
      transform: none;
      color: rgba(255, 255, 255, 0.5);
      background: none;
    }
  
    .menu-text {
      font-weight: 500;
    }
  
    .queue-status {
      position: fixed;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      background-color: rgba(0, 0, 0, 0.9);
      padding: 30px;
      border-radius: 12px;
      text-align: center;
      z-index: 1000;
      box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
      backdrop-filter: blur(8px);
      border: 1px solid rgba(255, 255, 255, 0.1);
    }
  
    .queue-status p {
      color: white;
      margin: 15px 0 0;
      font-size: 1.2rem;
      font-weight: 500;
      letter-spacing: 0.5px;
    }
  
    .queue-spinner {
      width: 40px;
      height: 40px;
      border: 3px solid rgba(255, 255, 255, 0.1);
      border-top-color: #538d4e;
      border-radius: 50%;
      margin: 0 auto;
      animation: spin 1s linear infinite;
    }
  
    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }
  
    .score-display {
      position: fixed;
      top: 20px;
      right: 20px;
      display: flex;
      align-items: center;
      gap: 10px;
      background-color: rgba(0, 0, 0, 0.8);
      padding: 8px 16px;
      border-radius: 8px;
      border: 1px solid rgba(255, 255, 255, 0.1);
      z-index: 1000;
    }
  
    .score-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;
    }
  
    .score-label {
      font-size: 0.8rem;
      color: #818384;
    }
  
    .score-value {
      font-size: 1.2rem;
      font-weight: bold;
      color: white;
    }
  
    .stats-text {
      font-size: 0.7rem;
      color: #818384;
    }
  
    .score-separator {
      color: #818384;
      font-size: 1.2rem;
      font-weight: bold;
    }
  </style>