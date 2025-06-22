import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import Home from './pages/Home';
import Game from './pages/Game';
import Matchmaking from './pages/Matchmaking';
import NavBar from './components/NavBar';
import './App.css'

function App() {
  return (
    <Router>
      <NavBar />
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/game/:id" element={<Game />} />
        <Route path="/matchmaking" element={<Matchmaking />} />
      </Routes>
    </Router>
  );
}

export default App;
