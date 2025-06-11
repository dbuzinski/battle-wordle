<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { browser } from '$app/environment';
  import { allowedGuesses } from '$lib/data/allowed_guesses';

  // Constants
  const WORD_LENGTH = 5;
  const MAX_GUESSES = 6;
  const MESSAGE_DURATION = 2000;
  const KEYBOARD_ROWS = [
    ['q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p'],
    ['a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l'],
    ['enter', 'z', 'x', 'c', 'v', 'b', 'n', 'm', 'backspace']
  ];

  // Game state
  let gameId: string;
  let socket: WebSocket;
  let playerId: string;
  let isJoining: boolean = false;
  let showMessage: boolean = false;
  let message: string = '';
  let turnStatusMessage: string = '';
  let messageTimeout: number;
  let isMyTurn: boolean = false;
  let isGameOver: boolean = false;
  let isSpectator: boolean = false;
  let gameResult: 'won' | 'lost' | null = null;
  let allGuesses: string[] = [];
  let statuses: string[][] = [];
  let solution: string = '';
  let currentGuess: string = '';
  let showCopiedMessage: boolean = false;
  let guesses = Array(MAX_GUESSES).fill(null).map(() => Array(WORD_LENGTH).fill(''));
  let currentGuessIndex = 0;
  let currentLetterIndex = 0;
  let letterStatuses = {};
  let playerIds: string[] = [];
  let isAnimating: boolean = false;
  let animationComplete: boolean = false;

  // Cookie management
  function setCookie(name: string, value: string) {
    document.cookie = `${name}=${value};path=/`;
  }

  function getCookie(name: string) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop()?.split(';').shift();
    return null;
  }

  // UI state
  function showMessageTemporarily(msg: string) {
    message = msg;
    showMessage = true;
    if (messageTimeout) clearTimeout(messageTimeout);
    messageTimeout = window.setTimeout(() => {
      showMessage = false;
    }, 3000);
  }

  function copyGameLink() {
    const gameUrl = `${window.location.origin}${window.location.pathname}?game=${gameId}`;
    navigator.clipboard.writeText(gameUrl).then(() => {
      showCopiedMessage = true;
      setTimeout(() => {
        showCopiedMessage = false;
      }, 2000);
    });
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
        } else if (isSpectator) {
          turnStatusMessage = "You are spectating this game";
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
    isAnimating = true;
    animationComplete = false;
    
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

    // Set a timeout to mark animation as complete after all tiles have flipped
    setTimeout(() => {
      isAnimating = false;
      animationComplete = true;
      
      // Update keyboard colors after animation completes
      allGuesses.forEach((guess, index) => {
        if (index < MAX_GUESSES) {
          const guessStatuses = getGuessStatuses(guess, solution);
          guessStatuses.forEach((status, i) => {
            const letter = guess[i].toUpperCase();
            const currentStatus = letterStatuses[letter];
            letterStatuses[letter] = mergeStatus(currentStatus, status);
          });
        }
      });
    }, 1000 + (WORD_LENGTH * 200)); // Increased base delay + time for all tiles to flip
  }

  // Add this new function to handle initial load
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
          showMessageTemporarily(`Cannot use letter ${letter} in position ${i + 1} - it was marked as present there`);
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
        showMessageTemporarily(`Must use letter ${requiredLetter} - it's in the word`);
        return false;
      }
    }
    
    // Check each letter against previous guesses
    for (let i = 0; i < guessArray.length; i++) {
      const letter = guessArray[i].toUpperCase();
      if (letterStatuses[letter] === 'absent') {
        showMessageTemporarily(`Cannot use letter ${letter} - it's not in the word`);
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
    const status = letterStatuses[upperKey];
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
          style="--tile-color: {getTileColor(i, j)}; --flip-delay: {j * 200}ms;"
        >
          <div class="tile-front">
            {letter.toUpperCase()}
          </div>
          <div class="tile-back">
            {letter.toUpperCase()}
          </div>
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
  :global(body) { background: #121213; margin: 0; }
  
  .tile {
    width: 50px; 
    height: 50px;
    position: relative;
    perspective: 1000px;
    transform-style: preserve-3d;
  }

  .tile-front,
  .tile-back {
    position: absolute;
    width: 100%;
    height: 100%;
    backface-visibility: hidden;
    display: flex;
    justify-content: center;
    align-items: center;
    font-weight: bold;
    font-size: 1.5rem;
    color: white;
    border: 2px solid #999;
    transition: transform 0.8s ease;
  }

  .tile-front {
    background-color: var(--tile-color, #121213);
    transform: rotateX(0deg);
  }

  .tile-back {
    background-color: #121213;
    transform: rotateX(180deg);
  }

  .tile.flipped .tile-front {
    transform: rotateX(180deg);
  }

  .tile.flipped .tile-back {
    transform: rotateX(0deg);
  }

  .tile.flipped {
    animation: flip 0.8s ease forwards;
    animation-delay: var(--flip-delay, 0ms);
  }

  @keyframes flip {
    0% {
      transform: rotateX(0);
    }
    100% {
      transform: rotateX(180deg);
    }
  }

  .keyboard {
    margin-top: 1rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
  }

  .kb-row { display: flex; gap: 6px; }

  .kb-key {
    padding: 10px; min-width: 36px;
    border: none; border-radius: 4px;
    font-weight: bold; color: white;
    text-transform: uppercase; cursor: pointer;
    transition: background-color 0.2s ease;
    background-color: #818384;
  }

  .kb-key:active { transform: scale(0.95); }
  
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
    top: 20px;
    left: 50%;
    transform: translateX(-50%);
    padding: 10px 20px;
    border-radius: 5px;
    background-color: #333;
    color: white;
    z-index: 1000;
    text-align: center;
  }

  .message.error {
    background-color: #ff4444;
  }

  .message.success {
    background-color: #44ff44;
    color: black;
  }

  .turn-status {
    text-align: center;
    color: white;
    font-size: 1.2em;
    margin: 1rem 0;
    padding: 0.5rem;
    background-color: rgba(255, 255, 255, 0.1);
    border-radius: 4px;
  }

  .turn-status.spectator {
    background-color: rgba(255, 255, 255, 0.2);
    font-style: italic;
  }

  .game-controls {
    display: flex;
    justify-content: center;
    margin: 2rem 0;
  }

  .game-over {
    position: fixed;
    top: 20px;
    left: 50%;
    transform: translateX(-50%);
    padding: 10px 20px;
    border-radius: 5px;
    background-color: #333;
    color: white;
    z-index: 1000;
    text-align: center;
  }

  .game-over h2 {
    margin: 0;
    font-size: 1.2rem;
  }

  .new-game-btn {
    padding: 0.75rem 1.5rem;
    background-color: #538d4e;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 1.1rem;
    transition: all 0.2s ease;
    font-weight: bold;
    min-width: 200px;
  }

  .new-game-btn:hover {
    background-color: #4a7d45;
    transform: translateY(-1px);
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
  }

  .new-game-btn:active {
    transform: translateY(0);
    box-shadow: none;
  }
</style>
