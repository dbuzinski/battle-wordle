import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';

const NavBar: React.FC = () => {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();

  const handleNav = (path: string) => {
    setOpen(false);
    navigate(path);
  };

  return (
    <>
      <button
        className="menu-icon"
        onClick={() => setOpen(o => !o)}
        aria-label="Open menu"
        aria-expanded={open}
        aria-controls="menu-sidebar"
        style={{ position: 'fixed', top: 20, left: 20, zIndex: 1002, background: 'none', border: 'none', padding: 0 }}
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
          <button className="menu-link" style={menuLinkStyle} onClick={() => handleNav('/game/new')}>New Game</button>
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