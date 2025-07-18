import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';

interface NavBarProps {
  isLoggedIn: boolean;
  playerName?: string;
  onLoginClick: () => void;
  onLogoutClick: () => void;
}

const NavBar: React.FC<NavBarProps> = ({ isLoggedIn, playerName, onLoginClick, onLogoutClick }) => {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();

  const handleNav = (path: string) => {
    setOpen(false);
    navigate(path);
  };

  return (
    <>
      {/* Header Bar */}
      <header style={{
        width: '100%',
        height: 64,
        background: '#1a1a1a',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        position: 'fixed',
        top: 0,
        left: 0,
        zIndex: 1100,
        borderBottom: '1px solid rgba(255,255,255,0.07)',
        padding: '0 1.5rem',
        boxSizing: 'border-box',
      }}>
        {/* Hamburger */}
        <button
          className="menu-icon"
          onClick={() => setOpen(o => !o)}
          aria-label="Open menu"
          aria-expanded={open}
          aria-controls="menu-sidebar"
          style={{ background: 'none', border: 'none', padding: 0 }}
        >
          <div className={`hamburger${open ? ' open' : ''}`} style={{ width: 30, height: 30, position: 'relative' }}>
            <div style={{
              position: 'absolute',
              width: '100%',
              height: 2,
              background: 'white',
              top: open ? '50%' : '25%',
              left: 0,
              transform: open ? 'translateY(-50%) rotate(45deg)' : 'none',
              transition: 'all 0.3s',
            }} />
            <div style={{
              position: 'absolute',
              width: '100%',
              height: 2,
              background: 'white',
              bottom: open ? '50%' : '25%',
              left: 0,
              transform: open ? 'translateY(50%) rotate(-45deg)' : 'none',
              transition: 'all 0.3s',
            }} />
          </div>
        </button>
        {/* Title */}
        <div className="navbar-title" style={{ flex: 1, textAlign: 'center', userSelect: 'none', minWidth: 0, overflow: 'hidden', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <img 
            src="/battlewordle-logo.png"
            alt="Battle Wordle" 
            style={{ 
              height: '40px', 
              width: 'auto', 
              maxWidth: '200px',
              objectFit: 'contain'
            }} 
          />
        </div>
        {/* Login/Logout */}
        <div className="navbar-actions" style={{ minWidth: 80, display: 'flex', justifyContent: 'flex-end', alignItems: 'center', gap: 8 }}>
          {isLoggedIn ? (
            <>
              <span className="navbar-player-name" style={{ color: '#f3f3f3', marginRight: 8, fontWeight: 500, maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'none' /* hidden by default, shown on larger screens */ }}>
                {playerName}
              </span>
              <button onClick={onLogoutClick} style={{ background: 'none', border: '1.5px solid #538d4e', color: '#538d4e', padding: '0.5rem 1rem', borderRadius: 4, cursor: 'pointer', fontWeight: 500, backgroundColor: 'rgba(83,141,78,0.08)' }}>Logout</button>
            </>
          ) : (
            <button onClick={onLoginClick} style={{ background: 'none', border: '1.5px solid #538d4e', color: '#538d4e', padding: '0.5rem 1rem', borderRadius: 4, cursor: 'pointer', fontWeight: 500, backgroundColor: 'rgba(83,141,78,0.08)' }}>Login</button>
          )}
        </div>
      </header>
      {/* Responsive styles */}
      <style>{`
        @media (max-width: 600px) {
          .navbar-title {
            font-size: 1.1rem !important;
          }
          .navbar-player-name {
            display: none !important;
          }
          .navbar-actions button {
            padding: 0.5rem 0.7rem !important;
            font-size: 0.95rem !important;
          }
        }
        @media (min-width: 601px) {
          .navbar-player-name {
            display: inline !important;
          }
        }
      `}</style>
      {/* Spacer for header */}
      <div style={{ height: 64 }} />
      {/* Overlay */}
      <div
        className={`menu-overlay${open ? ' open' : ''}`}
        onClick={() => setOpen(false)}
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          width: '100vw',
          height: '100vh',
          background: 'rgba(0,0,0,0.5)',
          opacity: open ? 1 : 0,
          visibility: open ? 'visible' : 'hidden',
          transition: 'all 0.3s',
          zIndex: 1000,
        }}
        aria-hidden={!open}
      />
      {/* Sidebar */}
      <nav
        className={`menu-sidebar${open ? ' open' : ''}`}
        id="menu-sidebar"
        style={{
          position: 'fixed',
          top: 0,
          left: open ? 0 : '-100vw',
          width: 300,
          height: '100%',
          background: '#1a1a1a',
          boxShadow: '2px 0 5px rgba(0,0,0,0.2)',
          transition: 'all 0.3s',
          zIndex: 1001,
          borderRight: '1px solid rgba(255,255,255,0.1)',
          padding: 20,
          display: 'flex',
          flexDirection: 'column',
        }}
        role="navigation"
        aria-label="Game menu"
      >
        <div style={{ marginBottom: 30, marginTop: 25, paddingBottom: 15 }}>
          <h3 style={{ display: 'none' }}>Menu</h3>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <button className="menu-link" style={menuLinkStyle} onClick={() => handleNav('/')}>Home</button>
          <button className="menu-link" style={menuLinkStyle} onClick={() => handleNav('/matchmaking')}>Find Match</button>
          <button className="menu-link" style={menuLinkStyle} onClick={() => handleNav('/challenge')}>Challenge</button>
          <button className="menu-link" style={menuLinkStyle} onClick={() => handleNav('/leaderboard')}>Leaderboard</button>
        </div>
      </nav>
    </>
  );
};

const menuLinkStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  padding: 16,
  color: 'white',
  textDecoration: 'none',
  borderRadius: 8,
  transition: 'all 0.2s',
  cursor: 'pointer',
  fontSize: '1.1rem',
  letterSpacing: '0.3px',
  background: 'none',
  border: 'none',
  width: '100%',
  textAlign: 'left',
  fontWeight: 500,
};

export default NavBar; 