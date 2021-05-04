import React from 'react';
import {
    Link,
    NavLink,
    useHistory
} from "react-router-dom";

import './Form.css';
import Status from './Status';

interface FormProps {
    inFlight: boolean
    submit(): void
}

export const Form: React.FunctionComponent<FormProps> = ({children, inFlight, submit}) => {
    const onSubmit = (e: React.FormEvent<HTMLFormElement>) => {
        e.stopPropagation();
        e.preventDefault();
        if (inFlight) {
            return
        }
        submit();
    };
    return <form className="form" onSubmit={onSubmit}>
        {children}
    </form>
};

interface TextProps {
    label: string
    to: string
}

interface StepProps {
    back: string
    next: string
    placeholder: string
    password?: boolean
    setState(value: string): void
    value: string
}

interface CustomStepProps {
    back: string
    next?: string
    showNext?: boolean
}

interface SubmitProps {
    ok: boolean
    inFlight: boolean
    submitted: boolean
}

export const Text: React.FunctionComponent<TextProps>= ({label, to}) => <div className="step">
    <Link to={to}>{label} <span className="arrow">&rarr;</span></Link>
</div>

export const Step: React.FunctionComponent<StepProps> = ({back, next, placeholder, password, setState, value}) => {
    let history = useHistory();
    const onKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === "Enter") {
            e.stopPropagation();
            e.preventDefault();
            if (value) {
                history.push(next);
            }
        }
    };
    const onChange = (e: React.ChangeEvent<HTMLInputElement>) => setState(e.target.value);
    return <div className="step">
        <Link to={back}>&larr;</Link> 
        <input
            type={password ? "password" : "text"}
            autoFocus
            placeholder={placeholder}
            onChange={onChange}
            onKeyPress={onKeyPress}
            value={value}
            autoComplete="off"
            autoCorrect="off"
            autoCapitalize="off"
            spellCheck="false"
        />
        { value && <Link to={next}>&rarr;</Link> }
    </div>;
}

export const CustomStep: React.FunctionComponent<CustomStepProps> = ({back, children, next, showNext}) => <div className="step">
    <NavLink style={{left: 0, position: "absolute"}} to={back}>&larr;</NavLink>
    {children}
    { next && showNext && <NavLink style={{right: 0, position: "absolute"}} to={next}>&rarr;</NavLink> }
</div>

export const Submit: React.FunctionComponent<SubmitProps> = ({inFlight, ok, submitted}) => <div style={{display: "flex"}}>
    <input autoFocus type="submit" value="Submit" />
    <Status done={submitted} inFlight={inFlight} ok={ok} />
</div>

export default Form;
