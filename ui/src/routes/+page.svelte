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
  let gameId;
  let socket;
  let playerId;
  let showMessage = false;
  let message = '';
  let turnStatusMessage = '';
  let messageTimeout;
  let isMyTurn = false;
  let isGameOver = false;
  let isSpectator = false;
  let isJoining = false;
  let gameResult = null;
  let allGuesses = [];
  let statuses = [];
  let solution = '';
  let guesses = Array(MAX_GUESSES).fill(null).map(() => Array(WORD_LENGTH).fill(''));
  let currentGuessIndex = 0;
  let currentLetterIndex = 0;
  let letterStatuses = {};
  let playerIds = [];

  // Cookie management
  function setCookie(name, value) {
    document.cookie = `${name}=${value};path=/`;
  }

  function getCookie(name) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop()?.split(';').shift();
    return null;
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

  function createNewGame() {
    gameId = crypto.randomUUID();
    window.history.pushState({}, '', `?game=${gameId}`);
    initializeWebSocket();
  }

  function startNewGame() {
    // Reset game state
    isGameOver = false;
    isMyTurn = false;
    solution = '';
    allGuesses = [];
    guesses = Array(MAX_GUESSES).fill(null).map(() => Array(WORD_LENGTH).fill(''));
    statuses = [];
    currentGuessIndex = 0;
    currentLetterIndex = 0;
    letterStatuses = {};
    gameResult = null;
    showMessage = false;
    message = '';
    
    // Create new game
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
      console.log('Received message:', msg);
      
      if (msg.type === 'game_state') {
        console.log('Processing game state:', {
          currentPlayer: msg.currentPlayer,
          playerId: playerId,
          isGameOver: msg.gameOver,
          solution: msg.solution ? 'present' : 'missing',
          guesses: msg.guesses,
          players: msg.players
        });

        // Update game state
        playerIds = msg.players || [];
        isSpectator = !playerIds.includes(playerId);
        isMyTurn = !isSpectator && msg.currentPlayer === playerId;
        isGameOver = msg.gameOver || false;
        
        if (msg.solution) {
          solution = msg.solution;
        }
        
        if (msg.guesses) {
          const oldGuessCount = allGuesses.length;
          allGuesses = msg.guesses;
          
          updateBoard();
        } else {
          // If this is the initial state, don't animate
          initializeBoard();
        }
        
        // Update turn status message
        if (isGameOver) {
          turnStatusMessage = '';
          // Set the appropriate game over message
          if (msg.loserId === playerId) {
            message = `You lost! You guessed the word "${msg.solution}"!`;
            gameResult = 'lost';
          } else if (msg.loserId) { // Only show won message if there is a loser
            message = `You won! Your opponent guessed the word "${msg.solution}"!`;
            gameResult = 'won';
          }
          showMessage = true;
        } else if (isSpectator) {
          turnStatusMessage = "You are spectating this game";
        } else if (msg.currentPlayer === 'waiting_for_opponent') {
          turnStatusMessage = "Waiting for opponent to join...";
          isMyTurn = false;
        } else {
          turnStatusMessage = isMyTurn ? "Your turn!" : "Waiting for opponent...";
        }
      } else if (msg.type === 'game_over') {
        console.log('Game over message received:', msg);
        isGameOver = true;
        isMyTurn = false;
        solution = msg.solution;
        allGuesses = msg.guesses || [];
        updateBoard();
        
        if (msg.loserId === playerId) {
          gameResult = 'lost';
          message = `You lost! You guessed the word "${msg.solution}"!`;
        } else {
          gameResult = 'won';
          message = `You won! Your opponent guessed the word "${msg.solution}"!`;
        }
        showMessage = true;
        turnStatusMessage = ''; // Clear turn status when game is over
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
    // Reset the board
    guesses = Array(MAX_GUESSES).fill(null).map(() => Array(WORD_LENGTH).fill(''));
    statuses = [];
    letterStatuses = {};
    
    // Update the board with all guesses
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
    }, 600); // Just wait for the last tile to finish (0.6s + 0.2s delay)
  }

  function initializeBoard() {
    // Don't animate on initial load
    updateBoard();
  }

  function validateGuess(guess) {
    const guessArray = guess.split('');

    if (!allowedGuesses.includes(guess)) {
      showMessageTemporarily("Not in word list");
      return false;
    }
    
    // First check if correct letters are in their correct positions
    for (let prevGuessIndex = 0; prevGuessIndex < allGuesses.length; prevGuessIndex++) {
      const prevGuess = allGuesses[prevGuessIndex];
      const prevStatuses = statuses[prevGuessIndex];
      
      for (let i = 0; i < prevGuess.length; i++) {
        if (prevStatuses[i] === 'correct') {
          const correctLetter = prevGuess[i].toUpperCase();
          if (guessArray[i].toUpperCase() !== correctLetter) {
            showMessageTemporarily(`Letter ${correctLetter} must be in position ${i + 1}`);
            return false;
          }
        }
      }
    }
    
    // Check if any letter is used in a position where it was previously marked as present
    for (let i = 0; i < guessArray.length; i++) {
      const letter = guessArray[i].toUpperCase();
      
      for (let prevGuessIndex = 0; prevGuessIndex < allGuesses.length; prevGuessIndex++) {
        const prevGuess = allGuesses[prevGuessIndex];
        const prevStatuses = statuses[prevGuessIndex];
        
        if (prevGuess[i].toUpperCase() === letter && prevStatuses[i] === 'present') {
          showMessageTemporarily(`Cannot use letter ${letter} in position ${i + 1}`);
          return false;
        }
      }
    }
    
    // Collect all required letters (correct and present)
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
    
    // Check if all required letters are used
    const usedLetters = new Set(guessArray.map(l => l.toUpperCase()));
    for (const requiredLetter of requiredLetters) {
      if (!usedLetters.has(requiredLetter)) {
        showMessageTemporarily(`Must use letter ${requiredLetter}`);
        return false;
      }
    }
    
    // Check each letter against previous guesses
    for (let i = 0; i < guessArray.length; i++) {
      const letter = guessArray[i].toUpperCase();
      if (letterStatuses[letter] === 'absent') {
        showMessageTemporarily(`Cannot use letter ${letter}`);
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
    // Only handle keyboard input if it's your turn and the game isn't over
    if (isGameOver || isSpectator) {
      console.log('Game is over or you are a spectator');
      return;
    }

    if (!isMyTurn) {
      console.log('Not your turn');
      return;
    }

    const key = e.key.toLowerCase();

    if (key === 'enter') {
      const guess = guesses[currentGuessIndex].join('');
      if (guess.length === WORD_LENGTH) {
        if (!validateGuess(guess)) {
          return;
        }
        console.log('Submitting guess:', guess);
        socket.send(JSON.stringify({
          type: 'guess',
          from: playerId,
          guess: guess
        }));
        // Clear the current guess
        guesses[currentGuessIndex] = Array(WORD_LENGTH).fill('');
        currentLetterIndex = 0;
      }
    } else if (key === 'backspace') {
      if (currentLetterIndex > 0) {
        currentLetterIndex--;
        guesses[currentGuessIndex][currentLetterIndex] = '';
      }
    } else if (/^[a-z]$/.test(key)) {
      if (currentLetterIndex < WORD_LENGTH) {
        guesses[currentGuessIndex][currentLetterIndex] = key;
        currentLetterIndex++;
      }
    }
  }

  function handleKeyPress(key) {
    handleKey({ key });
  }

  // WebSocket handling
  onMount(() => {
    if (browser) {
      // Get or create player ID - this is a persistent identity
      playerId = getCookie('playerId');
      if (!playerId) {
        playerId = crypto.randomUUID();
        setCookie('playerId', playerId);
      }

      // Get game ID from URL - this is specific to the current game
      gameId = $page.url.searchParams.get('game');
      if (gameId) {
        // If we have a game ID, connect to that game
        initializeWebSocket();
      } else {
        // If no game ID, create a new game
        createNewGame();
      }
    }
  });

  // UI helpers
  function getTileColor(row, col) {
    const status = statuses[row]?.[col];
    if (status === 'correct') return '#538d4e';
    if (status === 'present') return '#b59f3b';
    if (status === 'absent') return '#3a3a3c';
    return '#121213';
  }

  function getKeyColor(key) {
    if (key === 'enter' || key === 'backspace') return '#818384';
    const upperKey = key.toUpperCase();
    const status = letterStatuses[upperKey] || null;
    if (status === 'correct') return '#538d4e';
    if (status === 'present') return '#b59f3b';
    if (status === 'absent') return '#3a3a3c';
    return '#818384';
  }

  function handleJoin() {
    if (!gameId) {
      createNewGame();
    }
    
    isJoining = true;
    socket.send(JSON.stringify({
      type: 'join',
      from: playerId
    }));
  }
</script>

<svelte:window on:keydown={handleKey} />

<div class="container">
  <h1 style="text-align: center; color: white;">Battle Wordle!</h1>
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
      <h2>{message}</h2>
    </div>
  {/if}

  <div class="grid" style="display: grid; justify-content: center; grid-template-columns: repeat(5, 50px); gap: 5px;">
    {#each guesses as guess, i}
      {#each guess as letter, j}
        <div
          class="tile"
          class:flipped={statuses[i]?.[j]}
          class:current-row={i === currentGuessIndex}
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
              on:click={() => handleKeyPress(key)}
            >
              {key === 'enter' ? 'Enter' : '⌫'}
            </button>
          {:else}
            <button 
              class="kb-key" 
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

  <div class="game-controls">
    <button class="new-game-btn" on:click={startNewGame}>New Game</button>
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
    width: 50px; 
    height: 50px;
    display: flex;
    justify-content: center;
    align-items: center;
    font-weight: bold;
    font-size: 1.5rem;
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
    animation: flipTile 0.6s cubic-bezier(0.4, 0, 0.2, 1) forwards;
    animation-delay: calc(var(--col-index) * 0.1s);
  }

  @keyframes flipTile {
    0% {
      transform: rotateX(0);
    }
    100% {
      transform: rotateX(360deg);
    }
  }

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

  .tile.current-row {
    box-shadow: 0 0 10px rgba(255, 255, 255, 0.1);
  }

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
    gap: 0.5rem;
    padding: 1rem;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  }

  .kb-row { 
    display: flex; 
    gap: 6px;
    margin: 0.25rem 0;
  }

  .kb-key {
    padding: 12px;
    min-width: 40px;
    border: none;
    border-radius: 6px;
    font-weight: bold;
    color: white;
    text-transform: uppercase;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    background-color: #818384;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
    user-select: none;
  }

  .kb-key:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 8px rgba(0, 0, 0, 0.3);
  }

  .kb-key:active { 
    transform: translateY(1px);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  }
  
  .kb-key.correct {
    background-color: #538d4e;
    box-shadow: 0 2px 4px rgba(83, 141, 78, 0.3);
  }
  
  .kb-key.present {
    background-color: #b59f3b;
    box-shadow: 0 2px 4px rgba(181, 159, 59, 0.3);
  }
  
  .kb-key.absent {
    background-color: #3a3a3c;
    box-shadow: 0 2px 4px rgba(58, 58, 60, 0.3);
  }

  .message {
    position: fixed;
    top: 20px;
    left: 50%;
    transform: translateX(-50%);
    padding: 12px 24px;
    border-radius: 8px;
    background-color: rgba(51, 51, 51, 0.95);
    color: white;
    z-index: 1000;
    text-align: center;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
    animation: slideDown 0.3s ease-out;
  }

  @keyframes slideDown {
    from {
      transform: translate(-50%, -100%);
      opacity: 0;
    }
    to {
      transform: translate(-50%, 0);
      opacity: 1;
    }
  }

  .message.error {
    background-color: rgba(255, 68, 68, 0.95);
  }

  .message.success {
    background-color: rgba(68, 255, 68, 0.95);
    color: black;
  }

  .turn-status {
    text-align: center;
    color: white;
    font-size: 1.2em;
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

  .game-controls {
    display: flex;
    justify-content: center;
    margin: 2rem 0;
    gap: 1rem;
  }

  .game-over {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    padding: 2rem;
    border-radius: 12px;
    background-color: rgba(51, 51, 51, 0.95);
    color: white;
    z-index: 1000;
    text-align: center;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
    animation: fadeIn 0.5s ease-out;
    backdrop-filter: blur(8px);
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translate(-50%, -40%);
    }
    to {
      opacity: 1;
      transform: translate(-50%, -50%);
    }
  }

  .game-over h2 {
    margin: 0;
    font-size: 1.5rem;
    margin-bottom: 1rem;
  }

  .new-game-btn {
    padding: 1rem 2rem;
    background-color: #538d4e;
    color: white;
    border: none;
    border-radius: 8px;
    cursor: pointer;
    font-size: 1.1rem;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    font-weight: bold;
    min-width: 200px;
    box-shadow: 0 4px 12px rgba(83, 141, 78, 0.3);
  }

  .new-game-btn:hover {
    background-color: #4a7d45;
    transform: translateY(-2px);
    box-shadow: 0 6px 16px rgba(83, 141, 78, 0.4);
  }

  .new-game-btn:active {
    transform: translateY(0);
    box-shadow: 0 2px 8px rgba(83, 141, 78, 0.3);
  }

  h1 {
    font-size: 2.5rem;
    margin-bottom: 0.5rem;
    background: linear-gradient(45deg, #538d4e, #b59f3b);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  }

  h2 {
    font-size: 1.2em;
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
</style>
