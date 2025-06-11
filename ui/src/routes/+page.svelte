<script>
  import { onMount } from 'svelte';


  function setCookie(name, value, days = 30) {
    const expires = new Date(Date.now() + days * 86400000).toUTCString();
    document.cookie = `${name}=${encodeURIComponent(value)}; expires=${expires}; path=/`;
  }

  function getCookie(name) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return decodeURIComponent(parts.pop().split(';').shift());
  }

  let socket;
  let playerId;
  let playerName;
  let isMyTurn = false;
  let solution = '';
  let opponentGuess = null;
  let gameStarted = false;
  let allGuesses = [];

  let currentPlayerId = '';
  const maxGuesses = 6;
  const wordLength = 5;

  let guesses = Array(maxGuesses).fill(null).map(() => Array(wordLength).fill(''));
  let statuses = Array(maxGuesses).fill(null);
  let currentGuessIndex = 0;
  let currentLetterIndex = 0;
  let flippingRow = -1;
  let shakingRow = -1;
  let gameOver = false;
  let previousGuesses = [];
  let letterStatuses = {};

  let revealedTiles = Array(maxGuesses).fill(null).map(() => Array(wordLength).fill(false));

  let message = '';
  let messageTimeout;
  let isGameOver = false;
  let gameResult = ''; // 'won' or 'lost'
  let showMessage = false;

  const rows = [
    ['q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p'],
    ['a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l'],
    ['enter', 'z', 'x', 'c', 'v', 'b', 'n', 'm', 'backspace']
  ];

  onMount(() => {
    playerId = getCookie("playerId");
    playerName = getCookie("playerName");

    if (!playerId) {
      playerId = crypto.randomUUID();
      setCookie("playerId", playerId);
    }

    if (!playerName) {
      playerName = prompt("Enter your name") || "Anonymous";
      setCookie("playerName", playerName);
    }

    socket = new WebSocket('ws://localhost:8080/ws');

    socket.onopen = () => {
      console.log('WebSocket connected');
      socket.send(JSON.stringify({
        type: 'join',
        from: playerId,
        name: playerName
      }));
    };

    socket.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      console.log('Received message:', msg);
      
      if (msg.type === 'game_state') {
        console.log('Processing game state update');
        isMyTurn = msg.currentPlayer === playerId;
        gameStarted = msg.gameStarted;
        currentPlayerId = msg.currentPlayer;
        
        // Update solution if provided
        if (msg.solution) {
          console.log('Updating solution:', msg.solution);
          solution = msg.solution;
        }
        
        // Update guesses
        if (msg.guesses) {
          console.log('Updating guesses:', msg.guesses);
          const oldGuessCount = allGuesses.length;
          allGuesses = msg.guesses;
          
          // If there's a new guess, trigger the animation
          if (allGuesses.length > oldGuessCount) {
            flippingRow = oldGuessCount;
            setTimeout(() => {
              flippingRow = -1;
            }, 1050);
          }
          
          updateBoard();
        }
        
        if (gameStarted) {
          message = isMyTurn ? "Your turn! Try to avoid guessing the word!" : "Waiting for opponent...";
        } else {
          message = "Waiting for opponent to join...";
        }
      } else if (msg.type === 'game_over') {
        console.log('Processing game over');
        isGameOver = true;
        gameOver = true;
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
      }
    };

    socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    socket.onclose = () => {
      console.log('WebSocket connection closed');
    };
  });

  $: if (message) {
    showMessage = true;
    const isTemporary = !gameOver && !message.toLowerCase().includes('win') && !message.toLowerCase().includes('game over');
    if (isTemporary) {
      setTimeout(() => {
        showMessage = false;
        message = '';
      }, 1000);
    }
  }

  function getGuessStatuses(guess, solution) {
    if (!solution) return Array(guess.length).fill('absent');
    
    console.log('Getting statuses for guess:', guess, 'against solution:', solution);
    const statuses = Array(guess.length).fill('absent');
    const solutionChars = solution.split('');
    const taken = Array(guess.length).fill(false);

    // First pass: mark correct letters
    for (let i = 0; i < guess.length; i++) {
      if (guess[i].toUpperCase() === solution[i]) {
        statuses[i] = 'correct';
        taken[i] = true;
        console.log(`Letter ${guess[i]} at position ${i} is correct`);
      }
    }

    // Second pass: mark present letters
    for (let i = 0; i < guess.length; i++) {
      if (statuses[i] === 'correct') continue;
      const idx = solutionChars.findIndex((c, j) => c === guess[i].toUpperCase() && !taken[j]);
      if (idx !== -1) {
        statuses[i] = 'present';
        taken[idx] = true;
        console.log(`Letter ${guess[i]} at position ${i} is present`);
      }
    }

    console.log('Final statuses:', statuses);
    return statuses;
  }

  function updateBoard() {
    console.log('Updating board with solution:', solution, 'and guesses:', allGuesses);
    
    // Reset the board
    guesses = Array(maxGuesses).fill(null).map(() => Array(wordLength).fill(''));
    statuses = Array(maxGuesses).fill(null);
    revealedTiles = Array(maxGuesses).fill(null).map(() => Array(wordLength).fill(false));
    letterStatuses = {};
    
    // Update the board with all guesses
    allGuesses.forEach((guess, index) => {
      if (index < maxGuesses) {
        guesses[index] = guess.split('');
        const guessStatuses = getGuessStatuses(guess, solution);
        statuses[index] = guessStatuses;
        revealedTiles[index] = Array(wordLength).fill(true);
        
        // Update letter statuses
        guessStatuses.forEach((status, i) => {
          const letter = guess[i].toUpperCase();
          const currentStatus = letterStatuses[letter];
          const newStatus = mergeStatus(currentStatus, status);
          letterStatuses[letter] = newStatus;
          console.log(`Letter ${letter} status updated: ${currentStatus} -> ${newStatus}`);
        });
      }
    });
    
    console.log('Final letter statuses:', letterStatuses);
    currentGuessIndex = allGuesses.length;
    currentLetterIndex = 0;
  }

  function validateGuess(guess) {
    const guessArray = guess.split('');
    
    // First check if correct letters are in their correct positions
    for (let prevGuessIndex = 0; prevGuessIndex < allGuesses.length; prevGuessIndex++) {
      const prevGuess = allGuesses[prevGuessIndex];
      const prevStatuses = statuses[prevGuessIndex];
      
      for (let i = 0; i < prevGuess.length; i++) {
        if (prevStatuses[i] === 'correct') {
          const correctLetter = prevGuess[i].toUpperCase();
          if (guessArray[i].toUpperCase() !== correctLetter) {
            message = `Letter ${correctLetter} must be in position ${i + 1}`;
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
          message = `Cannot use letter ${letter} in position ${i + 1} - it was marked as present there`;
          return false;
        }
      }
    }
    
    // Then collect all required letters (correct and present)
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
        message = `Must use letter ${requiredLetter} - it's in the word`;
        return false;
      }
    }
    
    // Check each letter against previous guesses
    for (let i = 0; i < guessArray.length; i++) {
      const letter = guessArray[i].toUpperCase();
      
      // Check if this letter was previously marked as absent
      if (letterStatuses[letter] === 'absent') {
        message = `Cannot use letter ${letter} - it's not in the word`;
        return false;
      }
    }
    
    return true;
  }

  function showMessageTemporarily(msg) {
    message = msg;
    showMessage = true;
    if (messageTimeout) clearTimeout(messageTimeout);
    messageTimeout = setTimeout(() => {
      if (!isGameOver) {  // Only hide if not game over
        showMessage = false;
      }
    }, 2000);
  }

  async function submitGuess() {
    if (!isMyTurn || isGameOver) {
      console.log('Cannot submit guess: not your turn or game is over');
      return;
    }
    
    const guess = guesses[currentGuessIndex].join('');
    if (guess.length !== wordLength) {
      showMessageTemporarily('Not enough letters');
      return;
    }

    // Validate the guess against previous guesses
    if (!validateGuess(guess)) {
      showMessageTemporarily(message);
      return;
    }

    console.log('Sending guess:', {
      type: 'guess',
      from: playerId,
      guess: guess
    });
    
    socket.send(JSON.stringify({
      type: 'guess',
      from: playerId,
      guess: guess
    }));
  }

  function handleKey(e) {
    if (isGameOver || !isMyTurn) {
      console.log('Cannot handle key: game is over or not your turn');
      return;
    }

    const key = e.key.toLowerCase();

    if (key === 'enter') {
      submitGuess();
    } else if (key === 'backspace') {
      if (currentLetterIndex > 0) {
        currentLetterIndex--;
        guesses[currentGuessIndex][currentLetterIndex] = '';
      }
    } else if (/^[a-z]$/.test(key)) {
      if (currentLetterIndex < wordLength) {
        guesses[currentGuessIndex][currentLetterIndex] = key;
        currentLetterIndex++;
      }
    }
  }

  function handleKeyPress(key) {
    handleKey({ key });
  }

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
    console.log(`Getting color for key ${upperKey}:`, status);
    if (status === 'correct') return '#538d4e';
    if (status === 'present') return '#b59f3b';
    if (status === 'absent') return '#3a3a3c';
    return '#818384';
  }

  function mergeStatus(current, newStatus) {
    console.log(`Merging statuses: ${current} + ${newStatus}`);
    if (!current) return newStatus;
    if (current === 'correct') return 'correct';
    if (current === 'present' && newStatus === 'correct') return 'correct';
    if (current === 'absent' && (newStatus === 'present' || newStatus === 'correct')) return newStatus;
    return current;
  }
</script>

<svelte:window on:keydown={handleKey} />

<h1 style="text-align: center; color: white;">Battle Wordle!</h1>
<h2 style="text-align: center; color: white; font-size: 1.2em;">Try to avoid guessing the word!</h2>

{#if showMessage}
  <div class="message-popover" class:game-over={isGameOver}>
    {message}
  </div>
{/if}

<div class="grid" style="display: grid; justify-content: center; grid-template-columns: repeat(5, 50px); gap: 5px;">
  {#each guesses as guess, i}
    {#each guess as letter, j}
      <div
        class="tile {flippingRow === i ? 'flipping' : ''} {revealedTiles[i][j] ? 'revealed' : ''} {shakingRow === i ? 'shake' : ''} {statuses[i]?.[j] || ''}"
        style="--flip-bg-before: #121213; --flip-bg-after: {getTileColor(i, j)}; animation-delay: {flippingRow === i ? j * 0.15 : 0}s;"
      >
        {letter.toUpperCase()}
      </div>
    {/each}
  {/each}
</div>

<div class="keyboard">
  {#each rows as row}
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

<style>
  :global(body) { background: #121213; margin: 0; }
  .tile {
    width: 50px; height: 50px;
    display: flex; justify-content: center; align-items: center;
    font-weight: bold; font-size: 1.5rem; color: white;
    background-color: var(--flip-bg-before, #121213);
    border: 2px solid #999; backface-visibility: hidden;
    transform-style: preserve-3d;
  }
  .tile.flipping {
    animation: flip 0.6s ease-in-out forwards;
  }
  .tile.revealed {
    background-color: var(--flip-bg-after);
  }
  .tile.correct {
    --flip-bg-after: #538d4e;
  }
  .tile.present {
    --flip-bg-after: #b59f3b;
  }
  .tile.absent {
    --flip-bg-after: #3a3a3c;
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

  .message-popover {
    position: fixed;
    top: 20%;
    left: 50%;
    transform: translate(-50%, -50%);
    background-color: #f8f8f8;
    color: black;
    padding: 1rem 2rem;
    border-radius: 10px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
    font-size: 1.2rem;
    z-index: 1000;
    animation: fadeIn 0.3s ease-in-out;
  }

  .message-popover.game-over {
    background-color: #538d4e;
    color: white;
    font-size: 1.5rem;
    font-weight: bold;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translate(-50%, -60%); }
    to { opacity: 1; transform: translate(-50%, -50%); }
  }

  .key:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  @keyframes flip {
    0%   { transform: rotateX(0); background-color: var(--flip-bg-before); }
    50%  { transform: rotateX(90deg); }
    100% { transform: rotateX(0); background-color: var(--flip-bg-after); }
  }

  @keyframes shake {
    0%, 100% { transform: translateX(0); }
    25% { transform: translateX(-6px); }
    50% { transform: translateX(6px); }
    75% { transform: translateX(-6px); }
  }

  .shake { animation: shake 0.6s ease-in-out; }
</style>
