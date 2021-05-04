import React, {CSSProperties} from 'react';
import './Spinner.css';

interface SpinnerProps {
    style?: CSSProperties;
}

export const Spinner: React.FunctionComponent<SpinnerProps> = ({style}) => <div style={style} className='spinner-container'>
    <div className="spinner-path">
        <div className="spinlet"></div>
        <div className="spinlet"></div>
        <div className="spinlet"></div>
        <div className="spinlet"></div>
    </div>
    <svg xmlns="http://www.w3.org/2000/svg" version="1.1">
        <defs>
            <filter id="gooey">
                <feGaussianBlur in="SourceGraphic" stdDeviation="10" result="blur" />
                <feColorMatrix in="blur" mode="matrix" values="1 0 0 0 0  0 1 0 0 0  0 0 1 0 0  0 0 0 21 -7" result="goo" />
                <feBlend in="SourceGraphic" in2="goo" />
            </filter>
        </defs>
    </svg>
</div>

export default Spinner;
