import React, { useState, Dispatch, SetStateAction } from 'react';
import {
    TransitionGroup,
    CSSTransition
} from "react-transition-group";
import {
    Switch,
    Route,
    useLocation
} from "react-router-dom";
import './App.css';
import './fonts.css';
import {CustomStep, Form, Step, Submit, Text} from './Form';
import {Check, CheckGroup, TrueCheck, retryCheck} from './Check';
import client, {isError, Configuration, SystemdResult, SystemdSubState} from './api';

declare global { interface Window { configuration: Configuration; } };

type StatePair<T> = [T, Dispatch<SetStateAction<T>>];

const App: React.FunctionComponent = () => {
    let location = useLocation();
    const c = window.configuration;
    const states: StatePair<string>[] = []; 
    for (let i = 0; i < c.values.length; i++) {
        // eslint-disable-next-line react-hooks/rules-of-hooks
        states.push(useState(""));
    };
    let firstStep = "/submit";
    let lastStep = "/";
    const steps = c.values.map((v, i) => {
        if (i === 0) {
            firstStep = "/" + v.name;
        }
        if (i === c.values.length - 1) {
            lastStep = "/" + v.name;
        }
        let back = "/";
        if (i > 0) {
            back = "/" + c.values[i-1].name;
        }
        let next = "/submit";
        if (i < c.values.length - 1) {
            next = "/" + c.values[i+1].name;
        }
        return <Route path={"/" + v.name}>
            <Step value={states[i][0]} back={back} next={next} setState={states[i][1]} placeholder={v.description} password={v.secret} />
        </Route>;
    });
    let checks: React.ReactElement[] = [];
    c.checks.forEach((ch, i) => {
        if (!ch) {
            return;
        }
        if (ch.dns) {
            for (let j = 0; j < c.values.length; j++) {
                if (ch.dns.value === c.values[j].name) {
                    checks.push(<Check name="Testing DNS" check={retryCheck(() => {return client.dns(states[j][0]).then(r => {return !isError(r)})})} />);
                }
            }
        } else if (ch.systemd) {
            checks.push(<Check name={ch.systemd.description} check={retryCheck(() => {return client.systemd(ch.systemd!.unit).then(r => {return !isError(r) && r.result === SystemdResult.Success && r.subState === SystemdSubState.Dead})}, 10)} />);
        }
    });
    checks = [
        <Check name="Bringing Up Network" check={retryCheck(() => {return client.link().then(r => {
            console.log(isError(r));
            return !isError(r) && r.state === "up";
        }).catch((e) => {
            if (e instanceof TypeError && e.message === "Failed to fetch") {
                return false;
            }
            return Promise.reject(e);         
        })})} />,
        <Check name="Getting IP Address" check={retryCheck(() => {return client.link().then(r => {return !isError(r) && r.addresses.length !== 0})})} />,
        ...checks,
        <TrueCheck name="Done" check={() => Promise.resolve(true)} />,
    ];
    const [inFlight, setInFlight] = useState(false);
    const [submitted, setSubmited] = useState(false);
    const [submitOK, setSubmitOK] = useState(false);
    const submit = (): void => {
        setInFlight(true);
        const state = new Map<string, string>();
        c.values.forEach((v, i) => {
            state.set(v.name, states[i][0]);
        });
        client.onboard(JSON.stringify(Object.fromEntries(state.entries()))).then(r => {
            setSubmited(true)
            setSubmitOK(!isError(r));
            setInFlight(false);
        }).catch((e) => {
            setSubmited(true)
            setSubmitOK(false);
            setInFlight(false);
        });
    };
    if ((location.pathname !== "/submit") && submitted) {
        setSubmited(false);
        setSubmitOK(false);
    }
    return <div className="app">
        <h1 className="header line">Onboard</h1>
        <Form inFlight={inFlight} submit={submit}>
            <TransitionGroup>
                <CSSTransition
                  key={location.key}
                  classNames="blur"
                  timeout={300}
                >
                    <Switch location={location}>
                        <Route exact path="/">
                            <Text label="Get Started" to={firstStep} />
                        </Route>
                        { steps }
                        <Route exact path="/submit">
                            <CustomStep back={lastStep}>
                                <TransitionGroup>
                                    <CSSTransition
                                     key={submitted && submitOK ? "checks" : "submit"}
                                     classNames="blur"
                                     timeout={300}
                                    >
                                        <div style={{display: "flex", justifyContent: "center"}}>
                                            {(!submitted || !submitOK) && <Submit inFlight={inFlight} ok={submitOK} submitted={submitted} />}
                                            {(submitted && submitOK) && <CheckGroup>
                                                {checks}
                                            </CheckGroup>}
                                        </div>
                                    </CSSTransition>
                                </TransitionGroup>
                            </CustomStep>
                        </Route>
                    </Switch>
                </CSSTransition>
            </TransitionGroup>
        </Form>
    </div>
};

export default App;
