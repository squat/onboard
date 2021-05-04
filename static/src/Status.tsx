import React, {CSSProperties} from 'react';
import {
    TransitionGroup,
    CSSTransition
} from "react-transition-group";
import Spinner from './Spinner';
import './Status.css';

interface StatusProps {
    done: boolean
    inFlight: boolean
    ok: boolean
    style?: CSSProperties
}

export const Status: React.FunctionComponent<StatusProps> = ({done, inFlight, ok, style}) => <div className="status" style={style}>
    <TransitionGroup>
        <CSSTransition
          key={inFlight ? "inFlight" : (!done ? "!submitted" : (ok ? "ok" : "!ok"))}
          classNames="blur"
          timeout={300}
        >
            <div>
                { inFlight && <Spinner style={{fontSize: ".15em"}} /> }
                { done && (ok ? <span className="status-success"></span> : <span className="status-fail">&otimes;</span>) }
            </div>
        </CSSTransition>
    </TransitionGroup>
</div>

export default Status;
