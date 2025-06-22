<script>
  import { onMount } from 'svelte';
  import { browser } from '$app/environment';
  import { goto } from '$app/navigation';

  let playerId = '';
  let isInQueue = false;
  let queueSocket = null;

  function getCookie(name) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) {
      const cookieValue = parts.pop()?.split(';').shift();
      return cookieValue;
    }
    return null;
  }

  function findMatch() {
    if (isInQueue) return;
    
    isInQueue = true;
    
    const wsUrl = `${import.meta.env.VITE_WS_URL}`;
    
    queueSocket = new WebSocket(wsUrl);
    
    queueSocket.onopen = () => {
      queueSocket.send(JSON.stringify({
        type: 'queue',
        from: playerId
      }));
    };

    queueSocket.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      
      if (msg.type === 'match_found') {
        isInQueue = false;
        const gameId = msg.gameId;
        
        queueSocket.close();
        queueSocket = null;
        
        goto(`/game/${gameId}`);
      }
    };

    queueSocket.onerror = (error) => {
      console.error('Queue WebSocket error:', error);
      isInQueue = false;
      queueSocket = null;
    };

    queueSocket.onclose = () => {
      isInQueue = false;
      queueSocket = null;
    };
  }

  onMount(() => {
    if (browser) {
      playerId = getCookie('playerId');
      if (!playerId) {
        goto('/');
        return;
      }
      findMatch();
    }
  });
</script>

<div class="container">
  <div class="queue-status">
    <div class="queue-spinner"></div>
    <p>Finding match...</p>
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
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
  }

  .queue-status {
    background-color: rgba(0, 0, 0, 0.9);
    padding: 30px;
    border-radius: 12px;
    text-align: center;
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
</style> 