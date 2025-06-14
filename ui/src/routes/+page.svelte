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
    playerIds = msg.players || [];
    isSpectator = !playerIds.includes(playerId);
    isMyTurn = !isSpectator && msg.currentPlayer === playerId;
    isGameOver = msg.gameOver || false;
    
    if (msg.solution) {
      solution = msg.solution;
    }
    
    if (msg.guesses) {
      allGuesses = msg.guesses;
      updateBoard();
    } else {
      initializeBoard();
    }
    
    updateTurnStatus(msg);
  }

  function handleGameOver(msg) {
    isGameOver = true;
    isMyTurn = false;
    solution = msg.solution;
    allGuesses = msg.guesses || [];
    updateBoard();
    
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
    if (isGameOver || isSpectator || !isMyTurn) return;

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

  // Initialize game
  onMount(() => {
    if (browser) {
      playerId = getCookie('playerId') || crypto.randomUUID();
      if (!playerId) {
        setCookie('playerId', playerId);
      }

      gameId = $page.url.searchParams.get('game');
      if (gameId) {
        initializeWebSocket();
      } else {
        createNewGame();
      }
    }
  });
</script>

<svelte:window on:keydown={handleKey} />

<div class="container">
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

  <div class="game-controls">
    <button class="new-game-btn" on:click={startNewGame} type="button">New Game</button>
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
    padding: 12px 24px;
    border-radius: 8px;
    background-color: rgba(255, 255, 255, 0.95);
    color: #121213;
    z-index: 1000;
    text-align: center;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
    animation: slideDown 0.3s ease-out;
    backdrop-filter: blur(8px);
  }

  .game-over p {
    margin: 0;
    font-size: clamp(0.9rem, 2.5vw, 1.1rem);
    line-height: 1.5;
  }

  .game-over p:last-child {
    margin-top: 8px;
    font-weight: bold;
  }

  .new-game-btn {
    padding: clamp(0.8rem, 2vw, 1rem) clamp(1.5rem, 4vw, 2rem);
    background-color: #538d4e;
    color: white;
    border: none;
    border-radius: 8px;
    cursor: pointer;
    font-size: clamp(1rem, 2.5vw, 1.1rem);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    font-weight: bold;
    min-width: clamp(160px, 40vw, 200px);
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
</style>
