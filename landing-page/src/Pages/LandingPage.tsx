import React, {useEffect, useState} from "react";
import "@fortawesome/fontawesome-free/css/all.min.css";
import '../CSS/index.css';
import '../CSS/navbar.css';
import 'aos/dist/aos.css';
import AOS from "aos";
import androidlogo from '../assets/androiddown.png';
import ioslogo from '../assets/appledown.png';



const LandingPage: React.FC = () => {

  const [lastScrollY, setLastScrollY] = useState(0); // Track the last scroll position
  const [scrollDirection, setScrollDirection] = useState<'down' | 'up'>('down'); // Track the scroll direction

  
  useEffect(() => {
    AOS.init({
      duration: 1000, 
      once: true, 
    });
  }, []);

  const handleScroll = () => {
    const currentScrollY = window.scrollY;

    if (currentScrollY > lastScrollY) {
      setScrollDirection('down'); 
    } else {
      setScrollDirection('up'); 
    }

    setLastScrollY(currentScrollY);
  };

  
  useEffect(() => {
    window.addEventListener("scroll", handleScroll);

    return () => {
      window.removeEventListener("scroll", handleScroll);
    };
  }, [lastScrollY]);

  
  useEffect(() => {
    if (scrollDirection === 'down') {
      AOS.refresh(); 
    }
  }, [scrollDirection]);

  return (

    <div className="app">
      <div className="seccion1">
        <div className="contenedorbotones">
        <button className="image-button"><img src={ioslogo}></img></button>
        <button className="image-button1"><img src={androidlogo}></img></button>
        </div>
      </div>
      
      
      <hr className="separator-line"></hr>
      <div className="seccion2" data-aos="slide-left">
      

      </div>
      <hr className="separator-line"></hr>
      <div className="seccion3" data-aos="slide-up">
        
      </div>
    </div>
  );
};

export default LandingPage;
