import React, { Component } from 'react';

import Card from 'react-bootstrap/Card'
import Form from 'react-bootstrap/Form'
import Container from 'react-bootstrap/Container'
import Row from 'react-bootstrap/Row'
import Col from 'react-bootstrap/Col'
import Table from 'react-bootstrap/Table'
import Button from 'react-bootstrap/Button'

class StatusTable extends Component {
  createTable() {
    let nodeStatus = this.props.node;

    let table = [];

    let headers = [];
    headers.push(<th colSpan="2"></th>)
    for(let i = nodeStatus.LowWatermark ; i <= nodeStatus.HighWatermark ; i++) {
      headers.push(<th>{i}</th>);
    }

    table.push(<thead><tr>{headers}</tr></thead>);

    if(nodeStatus.Buckets != null) {
      let section = [];
      let first = true
      nodeStatus.Buckets.forEach(function(bucket) {
        let row = [];
        if(first) {
          row.push(<td rowSpan={nodeStatus.Buckets.length}>State Machine</td>);
          first = false;
        }
        row.push(<td>Bucket-{bucket.ID}</td>)
        bucket.Sequences.forEach(function(sequence) {
          if(sequence === 0){
            row.push(<td></td>);
          } else if(sequence === 1){
            row.push(<td style={{background:"yellow"}}>Q</td>);
          } else if(sequence === 2){
            row.push(<td style={{background:"yellow"}}>D</td>);
          } else if(sequence === 3){
            row.push(<td style={{background:"red"}}>I</td>);
          } else if(sequence === 4){
            row.push(<td style={{background:"yellow"}}>V</td>);
          } else if(sequence === 5){
            row.push(<td style={{background:"yellow"}}>P</td>);
          } else if(sequence === 6){
            row.push(<td style={{background:"green"}}>C</td>);
          }
        });
        section.push(<tr>{row}</tr>);
      });
      table.push(<tbody style={{border:"solid black 3px"}}>{section}</tbody>)
    }

    if(nodeStatus.Nodes != null) {

      nodeStatus.Nodes.forEach(function(node) {
        let section = [];
        let first = true;
        let length = node.BucketStatuses.length
        node.BucketStatuses.forEach(function(bucket) {
          let row = [];
          if(first) {
            row.push(<td rowSpan={length}>Node-{node.ID}</td>);
            first = false;
          }
          row.push(<td>Bucket-{bucket.BucketID}</td>)
          for(let i = nodeStatus.LowWatermark ; i <= nodeStatus.HighWatermark ; i++) {
            if(bucket.LastCheckpoint === i){
              row.push(<td>X</td>);
            } else if(bucket.LastCommit === i){
              row.push(<td>C</td>);
            } else if(bucket.LastPrepare === i){
              row.push(<td>P</td>);
            } else {
              row.push(<td></td>);
            }
          }
          section.push(<tr>{row}</tr>);
        });
        table.push(<tbody style={{border:"solid black 3px"}}>{section}</tbody>)
      });
    }

    return table;
  }

  render() {
    return <Table bordered size="sm">{this.createTable()}</Table>
  }
}

class NodeControl extends Component {

  constructor(props) {
    super(props);
    this.state = {
      automatic: true
    }
    this.switchProcessing = this.switchProcessing.bind(this)
  }
  

  handleProcess(e) {
    fetch("/node/"+this.props.node+"/process")
      .then(() => this.props.update())
      .catch(function(error) {
        console.log(error);
        alert(error);
      });
  }

  handlePropose(e) {
    fetch("/node/"+this.props.node+"/propose", {
      method:"POST",
      body: ""+Math.random()
    }).then(() => this.props.update())
      .catch(function(error) {
        console.log(error);
        alert(error);
      });
  }

  switchProcessing(e) {
    this.setState({automatic: e.target.value === "true"})
console.log("switching state for node "+this.props.node+" to "+e.target.value)
  }

  render() {
    if(this.state.automatic === true) {
      console.log("automatically processing for "+this.props.node+" as "+this.state.automatic)
      if(this.props.actions.total > 0) {
        this.handleProcess(null)
      }
    } else {
      console.log("NOT automatically processing for "+this.props.node+" as "+this.state.automatic)
    }

    return <Card>
      <Card.Body>
        <Card.Title>Node {this.props.node}</Card.Title>
        <Card.Text>
          <Table size="sm">
              <thead><th>Action</th><th>Outstanding</th></thead>
              <tbody>
                <tr><td>Broadcasts</td><td>{this.props.actions.broadcast}</td></tr>
                <tr><td>Unicasts</td><td>{this.props.actions.unicast}</td></tr>
                <tr><td>Preprocess</td><td>{this.props.actions.preprocess}</td></tr>
                <tr><td>Digest</td><td>{this.props.actions.digest}</td></tr>
                <tr><td>Validate</td><td>{this.props.actions.validate}</td></tr>
                <tr><td>Commit</td><td>{this.props.actions.commit}</td></tr>
                <tr><td>Checkpoint</td><td>{this.props.actions.checkpoint}</td></tr>
                <tr style={{fontWeight:"bold"}}><td>Total</td><td>{this.props.actions.total}</td></tr>
              </tbody>
          </Table>
        </Card.Text>
        <Form>
          <Form.Group>
            <Form.Label id='auto'>Processing </Form.Label>
            <Form.Control as="select" onChange={this.switchProcessing}>
              <option value="true">Automatic</option>
              <option value="false">Manual</option>
            </Form.Control>
          </Form.Group>
        </Form>
        <Button className="m-2" onClick={(e) => this.handleProcess(e)} disabled={this.props.actions.total === 0 || this.state.automatic === true}> Process </Button>
        <Button className="m-2" onClick={(e) => this.handlePropose(e)}> Propose </Button>
        </Card.Body>
      </Card>
  }
}

class App extends Component {
  constructor(props) {
    super(props);
    this.state = {
      node0: {
          actions: {},
          stateMachine: {}
      },
      node1: {
          actions: {},
          stateMachine: {}
      },
      node2: {
          actions: {},
          stateMachine: {}
      },
      node3: {
          actions: {},
          stateMachine: {}
      }
    };
  
    this.updateStatus = this.updateStatus.bind(this);
    this.updateStatus(0)
  }

  updateStatus() {
    fetch("/status")
      .then(response => response.json())
      .then(nodeStatuses => {
        this.setState(nodeStatuses);
      })
      .catch(function(error) {
        console.log(error);
        alert(error);
      });
  }

  render() {
    return (
      <div className="App">
        <header className="App-header">
          <Container className="m-1">
            <Row><Col><h1>MirBFT Status</h1></Col></Row>
            <Row>
              <Col><NodeControl node="0" actions={this.state.node0.actions} update={this.updateStatus}/></Col>
              <Col><NodeControl node="1" actions={this.state.node1.actions} update={this.updateStatus}/></Col>
              <Col><NodeControl node="2" actions={this.state.node2.actions} update={this.updateStatus}/></Col>
              <Col><NodeControl node="3" actions={this.state.node3.actions} update={this.updateStatus}/></Col>
            </Row>
            <Row><Col> <StatusTable node={this.state.node0.stateMachine}/> </Col></Row>
          </Container>
        </header>
      </div>
    );
  }
}

export default App;
