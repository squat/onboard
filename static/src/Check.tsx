import React, {CSSProperties, useEffect, useState} from 'react';
import {
    TransitionGroup,
    CSSTransition
} from "react-transition-group";
import Status from './Status';
import './Check.css';

interface check {
    (): Promise<boolean>
}

const timeout = (ms: number): Promise<void> => new Promise(resolve => setTimeout(resolve, ms)); 

export const retryCheck = (c: check, tries = 10, interval = 5000): check => {
    return async(): Promise<boolean> => {
        while (tries > 0) {
            const ok = await c();
            if (ok) {
                return Promise.resolve(ok);
            }
            tries--;
            if (tries <= 0) {
                break;
            }
            await timeout(interval)
        }
        return Promise.reject(new Error("timed out retrying check"));
    };
}

interface CheckProps {
    check: check
    details?: string
    expandable?: boolean
    name: string
    style?: CSSProperties
}

export const TrueCheck: React.FunctionComponent<CheckProps> = ({name, style}) => <div style={style} className="check">{name}</div>;

export const Check: React.FunctionComponent<CheckProps> = ({check, details, expandable, name, style}) => {
    const [inFlight, setInFlight] = useState(false);
    const [ok, setOK] = useState(false);
    const [err, setErr] = useState(Error);
    useEffect(() => {
        setInFlight(true);
        check().then((ok: boolean) => {
            setOK(ok);
            setInFlight(false);
        }).catch((e) => {
            setOK(false);
            setErr(e);
            setInFlight(false);
        });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return <div style={style} className="check">
        {name} <Status done={!inFlight} inFlight={inFlight} ok={ok} />
        {(err.message.length !== 0) &&
            <TransitionGroup component={null}>
                <CSSTransition appear={true} classNames="blur" timeout={300}>
                    <span className="check-error">{err.message}</span>
                </CSSTransition>
            </TransitionGroup>
        }
    </div>
};

interface CheckGroupProps {
    children?: React.ReactElement<CheckProps>[] | React.ReactElement<CheckProps>
}

const isCheck = (c: React.ReactElement): c is React.ReactElement<CheckProps> => {
    return c.type === Check || c.type === TrueCheck;
};

export const CheckGroup: React.FunctionComponent<CheckGroupProps> = ({children}) => {
    const [okCount, setOKCount] = useState(0);
    const checks: check[] = [];
    let i = 0;
    const filtered = React.Children.map(children, (c) => {
        if (((i === 0) || (i <= okCount)) && React.isValidElement(c) && isCheck(c)) {
            i++;
            checks.push(() => {
                return c.props.check().then((ok: boolean) => {
                    ok && setOKCount(i);
                    return ok;
                })
            })
            return c;
        }
        return undefined;
    }) || [];

    return <div className="check-group">
        {
            React.Children.map(filtered, (c, i) => {
                const props: {check: check, style: CSSProperties} = {
                    check: checks[i],
                    style: {
                        transform: "translateY(-"+((checks.length-1)*50).toString()+"%)",
                        opacity: 1-(checks.length-1-i)/4,
                    },
                };
                return  <TransitionGroup component={null}>
                        <CSSTransition appear={true} classNames="blur" timeout={300}>
                            {c && React.cloneElement(c, props)}
                    </CSSTransition>
                </TransitionGroup>;
            })
        }
    </div>
};
