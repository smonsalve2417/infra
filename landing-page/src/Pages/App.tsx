
import React from 'react';
import { BrowserRouter as Router, Routes, Route, Link, BrowserRouter } from 'react-router-dom';
import TermsAndService from './Terms&Service'; 
import LandingPage from './LandingPage'; 
import '../CSS/navbar.css';
import '../CSS/index.css';
import logo from '../assets/ESKIWI.svg'; 
import Gema from '../assets/Gemaeskiwi.svg';
import Button from '../Components/ButtonNav';
import '../CSS/ButtonNav.css'
import '../CSS/Terms.css'


    
const App: React.FC = () => {
    const handleClick = () => {
      };

    return (
    <BrowserRouter>
        <div className="app">
            <header className="header">
              
              <div className='navbar-content'>
              <ul className="menu">
                <li className="menu-item">
                  <Link to="/"><Button className='btn' label="ABOUT" onClick={handleClick}/></Link>
                </li>
                <li>
                  <Link to="/">
                    <img src={Gema} className="gema" alt="gema" />
                  </Link>
                </li>
                <li></li>
                <li className="menu-item">
                  <Link to="/terms"><Button className='btn1' label="TOS" onClick={handleClick}/></Link>
                </li>
              </ul>
              </div>
            </header>
            <Routes>
              <Route path="/" element={<LandingPage />} /> 
              <Route path="/terms" element={<TermsAndService />} />
            </Routes>
          </div>
        
      </BrowserRouter>
    );
  };
  
  export default App;