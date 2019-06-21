import React, { Component } from 'react';

import Card from 'react-bootstrap/Card'
 // import Collapse from 'react-bootstrap/Collapse'
import Form from 'react-bootstrap/Form'
import Container from 'react-bootstrap/Container'
import Row from 'react-bootstrap/Row'
import Col from 'react-bootstrap/Col'
import Table from 'react-bootstrap/Table'
import Button from 'react-bootstrap/Button'

/*
class NodeStatusTableData extends Component {
  constructor(props) {
    super(props);
    this.state = {
      collapsed: true
    }
  }

  createTable() {
    let obj = this
    let nodeStatus = this.props.nodeStatus;

    let table = [];


    if(nodeStatus.Buckets != null) {
      let section = [];
      let first = true
      nodeStatus.Buckets.forEach(function(bucket) {
        let row = [];
        if(first) {
          row.push(<td rowSpan={nodeStatus.Buckets.length} style={{verticalAlign:"middle"}}>
            <Button onClick={() => {obj.setState({collapsed: !obj.state.collapsed})}} aria-controls="{obj.props.node}-msg" aria-expanded="{!obj.state.collapsed}">Node-{obj.props.node} State Machine</Button>
          </td>);
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
        table.push(<Collapse id="{obj.props.node}-msgs" in={!obj.state.collapsed}><tbody style={{border:"solid black 3px"}}>{section}</tbody></Collapse>)
      });
    }

    return table;
  }
}
*/

class SequenceHeaders extends Component {
  render() {
    return <thead>
      <tr>
        <th colSpan="2" key="unused"></th>
        {[...Array(this.props.highWatermark - this.props.lowWatermark + 1).keys()].map((i) => {
          return <th key={"header-"+i}>{i+this.props.lowWatermark}</th>;
        })}
      </tr>
    </thead>
  }
}

class SequenceStatusRow extends Component {
  render() {
    let count = 0;

    return <tr>
      {(() => {if(this.props.bucket.ID === 0) {
        return <td rowSpan={this.props.numBuckets} style={{verticalAlign:"middle"}}>
          <Button>Node-{this.props.nodeID} State Machine</Button>
        </td>
      }})()}

      <td>Bucket-{this.props.bucket.ID}</td>

      {[...Array(this.props.offset).keys()].map((i) => {
          return <td key={"offset-"+i} style={{background:"black"}}></td>;
      })}

      {
        this.props.bucket.Sequences.map((sequence) => {
        count++;
        if(sequence === 0){
          return <td key={count}></td>;
        } else if(sequence === 1){
          return <td  key={count} style={{background:"yellow"}}>Q</td>;
        } else if(sequence === 2){
          return <td  key={count} style={{background:"yellow"}}>D</td>;
        } else if(sequence === 3){
          return <td  key={count} style={{background:"red"}}>I</td>;
        } else if(sequence === 4){
          return <td  key={count} style={{background:"yellow"}}>V</td>;
        } else if(sequence === 5){
          return <td  key={count} style={{background:"yellow"}}>P</td>;
        } else if(sequence === 6){
          return <td  key={count} style={{background:"green"}}>C</td>;
        } else {
          return <td key={count}>?</td>
        }
      })}

      {[...Array(this.props.padding).keys()].map((i) => {
        return <td key={"padding-"+i} style={{background:"gray"}}></td>
      })}
    </tr>
  }
}

class SequenceStatusBody extends Component {
  render() {
    return <tbody style={{border:"solid black 3px"}}>
      {this.props.buckets.map((bucket) => {
        return <SequenceStatusRow key={"node-"+this.props.nodeID+"-"+bucket.ID} bucket={bucket} offset={this.props.offset} padding={this.props.padding} numBuckets={this.props.buckets.length} nodeID={this.props.nodeID}/>
      })}
    </tbody>
  }
}

class StatusTable extends Component {
  render() {
    let lowWatermark = Math.min(...(this.props.nodes.map((i) => {return i.stateMachine.LowWatermark})))
    let highWatermark = Math.max(...(this.props.nodes.map((i) => {return i.stateMachine.HighWatermark})))
    if(highWatermark <= lowWatermark) {
      return null
    }

    return <Table bordered size="sm">
       <SequenceHeaders lowWatermark={lowWatermark} highWatermark={highWatermark} />
       {this.props.nodes.map((node) => {
         return <SequenceStatusBody key={"node-"+node.ID} nodeID={node.ID} buckets={node.stateMachine.Buckets} offset={node.stateMachine.LowWatermark - lowWatermark} padding={highWatermark - node.stateMachine.HighWatermark } />
       })}
    </Table>
  }
}

class NodeControl extends Component {

  constructor(props) {
    super(props);
    this.state = {
      automatic: true
    }
    this.processing = false
    this.switchProcessing = this.switchProcessing.bind(this)
  }
  

  handleProcess(e) {
    if(this.processing === true) {
      return
    }
    this.processing = true;
    let delay = this.state.automatic
    if(this.state.automatic === "manual")  {
      delay = 0
    }

    setTimeout(() => {
      fetch("/node/"+this.props.node+"/process")
        .then(() => {
          this.processing = false
          this.props.update();
        })
        .catch(function(error) {
          console.log(error);
          alert(error);
        });
    }, delay)
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
    this.setState({automatic: e.target.value})
  }

  render() {
    if(this.props.actions.total > 0 && this.state.automatic !== "manual") {
      this.handleProcess(null)
    }

    return <Card>
      <Card.Body>
        <Card.Title>Node {this.props.node}</Card.Title>
          <Table size="sm">
              <thead><tr><th>Action</th><th>Outstanding</th></tr></thead>
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
        <Form>
          <Form.Group>
            <Form.Label id='auto'>Processing </Form.Label>
            <Form.Control as="select" onChange={this.switchProcessing}>
              <option value="0">Automatic (immediate)</option>
              <option value="50">Automatic (50ms delay)</option>
              <option value="500">Automatic (500ms delay)</option>
              <option value="1500">Automatic (1500ms delay)</option>
              <option value="manual">Manual</option>
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
    this.refreshing = false;
    this.state = {
      nodes: [],
      refreshing: false
    };
  
    this.updateStatus = this.updateStatus.bind(this);
    this.updateStatus(0);
  }

  updateStatus() {
    if( this.refreshing === true) {
      return
    }
    this.refreshing= true
    fetch("/status")
      .then(response => response.json())
      .then(nodeStatuses => {
        this.setState({
          nodes: nodeStatuses
        });
        this.refreshing = false
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
              {this.state.nodes.map((node) => {
                return <Col key={"node-"+node.ID}><NodeControl key={"node-"+node.ID} node={node.ID} actions={node.actions} update={this.updateStatus}/></Col>
              })}
            </Row>
            <Row><Col> <StatusTable nodes={this.state.nodes}/> </Col></Row>
          </Container>
        </header>
      </div>
    );
  }
}

export default App;
